#ifndef VMLENS_SOCKET_CAPTURE_H
#define VMLENS_SOCKET_CAPTURE_H

#include "../common/flow_defs.h"
#include "../common/flow_event.h"

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

#endif // VMLENS_SOCKET_CAPTURE_H
