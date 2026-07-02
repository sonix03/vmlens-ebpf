//go:build ignore

// SPDX-License-Identifier: GPL-2.0
// Metadata-only flow tracker. It never reads packet payloads or user buffers.
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#define DIR_INGRESS 1
#define DIR_EGRESS 2
#define IPPROTO_TCP_VALUE 6
#define IPPROTO_UDP_VALUE 17

struct flow_event {
    __u64 timestamp_ns;
    __u64 bytes;
    __u32 src_addr;
    __u32 dst_addr;
    __u32 connections;
    __u16 src_port;
    __u16 dst_port;
    __u8 protocol;
    __u8 direction;
};

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
    event->src_addr = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    event->dst_addr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
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

char LICENSE[] SEC("license") = "GPL";

