//go:build ignore

// SPDX-License-Identifier: GPL-2.0
// Metadata-only flow tracker. It never reads packet payloads or user buffers.
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#include "flow_defs.h"

struct flow_event {
    __u64 timestamp_ns;
    __u64 bytes;
    __u8 src_addr[16];
    __u8 dst_addr[16];
    __u32 connections;
    __u16 src_port;
    __u16 dst_port;
    __u16 family;
    __u8 protocol;
    __u8 direction;
    __u32 packets;
} __attribute__((packed));

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 32768);
    __type(key, __u64);
    __type(value, struct flow_event);
} pending_io SEC(".maps");

static __always_inline void socket_metadata(struct flow_event *event, struct sock *sk,
                                             __u8 protocol, __u8 direction)
{
    // Keep the local VM as source for both directions. Direction tells the
    // backend whether the bytes were sent or received by this agent.
    event->family = BPF_CORE_READ(sk, __sk_common.skc_family);
    if (event->family == AF_INET6_VALUE) {
        BPF_CORE_READ_INTO(&event->src_addr, sk, __sk_common.skc_v6_rcv_saddr);
        BPF_CORE_READ_INTO(&event->dst_addr, sk, __sk_common.skc_v6_daddr);
    } else {
        __u32 src_addr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
        __u32 dst_addr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
        __builtin_memcpy(event->src_addr, &src_addr, sizeof(src_addr));
        __builtin_memcpy(event->dst_addr, &dst_addr, sizeof(dst_addr));
        event->family = AF_INET_VALUE;
    }
    event->src_port = BPF_CORE_READ(sk, __sk_common.skc_num);
    event->dst_port = __builtin_bswap16(BPF_CORE_READ(sk, __sk_common.skc_dport));
    event->protocol = protocol;
    event->direction = direction;
    event->timestamp_ns = bpf_ktime_get_ns();
}

static __always_inline int emit_connection(struct sock *sk, __u8 protocol, __u8 direction)
{
    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return 0;
    __builtin_memset(event, 0, sizeof(*event));
    socket_metadata(event, sk, protocol, direction);
    event->connections = 1;
    bpf_ringbuf_submit(event, 0);
    return 0;
}

static __always_inline int remember_io(struct sock *sk, __u8 protocol, __u8 direction)
{
    __u64 key = bpf_get_current_pid_tgid();
    struct flow_event event = {};
    socket_metadata(&event, sk, protocol, direction);
    return bpf_map_update_elem(&pending_io, &key, &event, BPF_ANY);
}

static __always_inline int finish_io(struct pt_regs *ctx)
{
    __u64 key = bpf_get_current_pid_tgid();
    struct flow_event *saved = bpf_map_lookup_elem(&pending_io, &key);
    // TCP/UDP sendmsg and recvmsg return int. Ignore stale upper register bits.
    int result = (int)PT_REGS_RC(ctx);
    if (!saved) return 0;
    if (result > 0) {
        struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
        if (event) {
            __builtin_memcpy(event, saved, sizeof(*event));
            event->bytes = (__u64)result;
            bpf_ringbuf_submit(event, 0);
        }
    }
    bpf_map_delete_elem(&pending_io, &key);
    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(trace_tcp_connect, struct sock *sk) { return emit_connection(sk, IPPROTO_TCP_VALUE, DIR_EGRESS); }

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(trace_tcp_v6_connect, struct sock *sk) { return emit_connection(sk, IPPROTO_TCP_VALUE, DIR_EGRESS); }

SEC("kretprobe/inet_csk_accept")
int trace_tcp_accept(struct pt_regs *ctx)
{
    struct sock *sk = (struct sock *)PT_REGS_RC(ctx);
    return sk ? emit_connection(sk, IPPROTO_TCP_VALUE, DIR_INGRESS) : 0;
}

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_send, struct sock *sk) { return remember_io(sk, IPPROTO_TCP_VALUE, DIR_EGRESS); }
SEC("kretprobe/tcp_sendmsg")
int trace_tcp_send_ret(struct pt_regs *ctx) { return finish_io(ctx); }
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_tcp_recv, struct sock *sk) { return remember_io(sk, IPPROTO_TCP_VALUE, DIR_INGRESS); }
SEC("kretprobe/tcp_recvmsg")
int trace_tcp_recv_ret(struct pt_regs *ctx) { return finish_io(ctx); }

SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(trace_udp_send, struct sock *sk) { return remember_io(sk, IPPROTO_UDP_VALUE, DIR_EGRESS); }
SEC("kretprobe/udp_sendmsg")
int trace_udp_send_ret(struct pt_regs *ctx) { return finish_io(ctx); }
SEC("kprobe/udp_recvmsg")
int BPF_KPROBE(trace_udp_recv, struct sock *sk) { return remember_io(sk, IPPROTO_UDP_VALUE, DIR_INGRESS); }
SEC("kretprobe/udp_recvmsg")
int trace_udp_recv_ret(struct pt_regs *ctx) { return finish_io(ctx); }

static __always_inline int load_bytes(void *data, void *data_end, __u64 offset, void *target, __u64 size)
{
    if (data + offset + size > data_end) return 0;
    __builtin_memcpy(target, data + offset, size);
    return 1;
}

static __always_inline void copy_v4(__u8 destination[16], __u32 value)
{
    __builtin_memset(destination, 0, 16);
    __builtin_memcpy(destination, &value, sizeof(value));
}

static __always_inline int emit_tc_packet(struct __sk_buff *skb, __u8 direction)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    __u16 eth_proto = 0;
    __u64 network_offset = 14;
    __u8 protocol = 0;
    __u16 family = 0;
    __u16 src_port = 0;
    __u16 dst_port = 0;
    __u8 src_addr[16] = {};
    __u8 dst_addr[16] = {};
    __u8 tcp_flags = 0;

    if (!load_bytes(data, data_end, 12, &eth_proto, sizeof(eth_proto))) return TC_ACT_OK;
    eth_proto = bpf_ntohs(eth_proto);

    if (eth_proto == ETH_P_IP_VALUE) {
        __u8 version_ihl = 0;
        __u32 saddr = 0;
        __u32 daddr = 0;
        if (!load_bytes(data, data_end, network_offset, &version_ihl, sizeof(version_ihl))) return TC_ACT_OK;
        __u8 ihl = (version_ihl & 0x0f) * 4;
        if (ihl < 20) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + 9, &protocol, sizeof(protocol))) return TC_ACT_OK;
        if (protocol != IPPROTO_TCP_VALUE && protocol != IPPROTO_UDP_VALUE) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + 12, &saddr, sizeof(saddr))) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + 16, &daddr, sizeof(daddr))) return TC_ACT_OK;
        copy_v4(src_addr, saddr);
        copy_v4(dst_addr, daddr);
        network_offset += ihl;
        family = AF_INET_VALUE;
    } else if (eth_proto == ETH_P_IPV6_VALUE) {
        if (!load_bytes(data, data_end, network_offset + 6, &protocol, sizeof(protocol))) return TC_ACT_OK;
        if (protocol != IPPROTO_TCP_VALUE && protocol != IPPROTO_UDP_VALUE) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + 8, src_addr, 16)) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + 24, dst_addr, 16)) return TC_ACT_OK;
        network_offset += 40;
        family = AF_INET6_VALUE;
    } else {
        return TC_ACT_OK;
    }

    if (!load_bytes(data, data_end, network_offset, &src_port, sizeof(src_port))) return TC_ACT_OK;
    if (!load_bytes(data, data_end, network_offset + 2, &dst_port, sizeof(dst_port))) return TC_ACT_OK;
    src_port = bpf_ntohs(src_port);
    dst_port = bpf_ntohs(dst_port);
    if (protocol == IPPROTO_TCP_VALUE) {
        if (!load_bytes(data, data_end, network_offset + 13, &tcp_flags, sizeof(tcp_flags))) return TC_ACT_OK;
    }

    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return TC_ACT_OK;
    __builtin_memset(event, 0, sizeof(*event));
    event->timestamp_ns = bpf_ktime_get_ns();
    event->bytes = skb->len;
    event->family = family;
    event->protocol = protocol;
    event->direction = direction;
    event->packets = 1;
    event->connections = (protocol == IPPROTO_TCP_VALUE && (tcp_flags & TCP_SYN) && !(tcp_flags & TCP_ACK)) ? 1 : 0;

    if (direction == DIR_INGRESS) {
        __builtin_memcpy(event->src_addr, dst_addr, 16);
        __builtin_memcpy(event->dst_addr, src_addr, 16);
        event->src_port = dst_port;
        event->dst_port = src_port;
    } else {
        __builtin_memcpy(event->src_addr, src_addr, 16);
        __builtin_memcpy(event->dst_addr, dst_addr, 16);
        event->src_port = src_port;
        event->dst_port = dst_port;
    }
    bpf_ringbuf_submit(event, 0);
    return TC_ACT_OK;
}

SEC("tcx/ingress")
int tc_ingress(struct __sk_buff *skb) { return emit_tc_packet(skb, DIR_INGRESS); }

SEC("tcx/egress")
int tc_egress(struct __sk_buff *skb) { return emit_tc_packet(skb, DIR_EGRESS); }

char LICENSE[] SEC("license") = "GPL";
