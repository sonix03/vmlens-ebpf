#ifndef METRICS_RETRANSMIT_H
#define METRICS_RETRANSMIT_H

#include "../../../../../shared/bpf/flow_defs.h"
#include "../../../../../shared/bpf/flow_event.h"
#include "../../../../../shared/bpf/flow_maps.h"
#include "../connection/socket_capture.bpf.h"
#include "../rtt/rtt.bpf.h"

#define RETRANSMIT_SOURCE_TCP_RETRANSMIT_SKB 1

static __always_inline int emit_tcp_retransmission(struct sock *sk)
{
    if (!sk) {
        return 0;
    }

    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        count_capture_stat(CAPTURE_STAT_RINGBUF_FULL);
        return 0;
    }

    __builtin_memset(event, 0, sizeof(*event));
    socket_metadata(event, sk, IPPROTO_TCP_VALUE, DIR_EGRESS);
    event->retransmissions = 1;
    event->rtt_us = tcp_smoothed_rtt_us(sk);
    bpf_ringbuf_submit(event, 0);
    return 0;
}

#endif // METRICS_RETRANSMIT_H
