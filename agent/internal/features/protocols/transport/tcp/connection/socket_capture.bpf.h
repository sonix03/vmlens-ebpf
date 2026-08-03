#ifndef SOCKET_CAPTURE_H
#define SOCKET_CAPTURE_H

#include "../../../../../shared/bpf/flow_defs.h"
#include "../../../../../shared/bpf/flow_event.h"
#include "../../../../../shared/bpf/flow_maps.h"
#include "../../../../traffic/bytes/bytes.bpf.h"
#include "../../../../classification/ports.bpf.h"
#include "../../../application/delay/delay.bpf.h"
#include "request_response.bpf.h"

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
    set_ports(event,
              BPF_CORE_READ(sk, __sk_common.skc_num),
              __builtin_bswap16(BPF_CORE_READ(sk, __sk_common.skc_dport)));
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
    mark_connection(event);
    bpf_ringbuf_submit(event, 0);
    return 0;
}

static __always_inline int remember_io(struct sock *sk, __u8 protocol, __u8 direction)
{
    __u64 key = bpf_get_current_pid_tgid();
    struct flow_event event = {};
    socket_metadata(&event, sk, protocol, direction);

    // Application delay is a TCP request/response notion. A UDP socket can be
    // unconnected and shared across peers, so timing it would mix conversations.
    if (protocol == IPPROTO_TCP_VALUE) {
        __u64 sock_key = (__u64)sk;
        bpf_map_update_elem(&pending_sock, &key, &sock_key, BPF_ANY);
        if (direction == DIR_EGRESS) {
            mark_request_sent(sk);
        }
    }

    return bpf_map_update_elem(&pending_io, &key, &event, BPF_ANY);
}

static __always_inline int finish_io(struct pt_regs *ctx)
{
    __u64 key = bpf_get_current_pid_tgid();
    struct flow_event *saved = bpf_map_lookup_elem(&pending_io, &key);
    __u64 *sock_key = bpf_map_lookup_elem(&pending_sock, &key);
    // TCP/UDP sendmsg and recvmsg return int. Ignore stale upper register bits.
    int result = (int)PT_REGS_RC(ctx);
    if (!saved) {
        bpf_map_delete_elem(&pending_sock, &key);
        return 0;
    }
    if (result > 0) {
        struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
        if (event) {
            __builtin_memcpy(event, saved, sizeof(*event));
            set_bytes(event, (__u64)result);
            // Settle the delay here: this is the point where the response bytes
            // are known to have arrived, not merely where the call was entered.
            if (sock_key && saved->direction == DIR_INGRESS) {
                event->app_delay_us = take_response_delay_us((struct sock *)*sock_key);
            }
            bpf_ringbuf_submit(event, 0);
        }
    }
    bpf_map_delete_elem(&pending_io, &key);
    bpf_map_delete_elem(&pending_sock, &key);
    return 0;
}

#endif // SOCKET_CAPTURE_H
