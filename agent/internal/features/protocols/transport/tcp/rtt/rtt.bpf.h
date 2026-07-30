#ifndef METRICS_RTT_H
#define METRICS_RTT_H

#include "../../../../../shared/bpf/flow_defs.h"
#include "../../../../../shared/bpf/flow_event.h"
#include "../../../../../shared/bpf/flow_maps.h"
#include "../connection/socket_capture.bpf.h"

#define RTT_SOURCE_CONNECTIVITY_PROBE 1
#define RTT_SOURCE_TCP_SRTT 2
#define RTT_EMIT_INTERVAL_NS 1000000000ULL

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 32768);
    __type(key, __u64);
    __type(value, __u64);
} rtt_last_emit SEC(".maps");

static __always_inline __u32 tcp_smoothed_rtt_us(struct sock *sk)
{
    struct tcp_sock *tcp = (struct tcp_sock *)sk;
    __u32 srtt_us = BPF_CORE_READ(tcp, srtt_us);

    /*
     * Linux stores tcp_sock.srtt_us shifted left by 3 to keep fractional
     * precision. Shift it back so userspace receives real microseconds.
     */
    return srtt_us >> 3;
}

static __always_inline int should_emit_rtt_sample(struct sock *sk, __u64 now_ns)
{
    __u64 key = (__u64)sk;
    __u64 *last = bpf_map_lookup_elem(&rtt_last_emit, &key);

    if (last && now_ns - *last < RTT_EMIT_INTERVAL_NS) {
        return 0;
    }

    bpf_map_update_elem(&rtt_last_emit, &key, &now_ns, BPF_ANY);
    return 1;
}

static __always_inline int emit_tcp_rtt_sample(struct sock *sk)
{
    if (!sk) {
        return 0;
    }

    __u32 rtt_us = tcp_smoothed_rtt_us(sk);
    if (rtt_us == 0) {
        return 0;
    }

    __u64 now_ns = bpf_ktime_get_ns();
    if (!should_emit_rtt_sample(sk, now_ns)) {
        return 0;
    }

    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        count_capture_stat(CAPTURE_STAT_RINGBUF_FULL);
        return 0;
    }

    __builtin_memset(event, 0, sizeof(*event));
    socket_metadata(event, sk, IPPROTO_TCP_VALUE, DIR_EGRESS);
    event->timestamp_ns = now_ns;
    event->rtt_us = rtt_us;
    bpf_ringbuf_submit(event, 0);
    return 0;
}

#endif // METRICS_RTT_H
