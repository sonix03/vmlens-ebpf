#ifndef APPLICATION_DELAY_H
#define APPLICATION_DELAY_H

#include "../../../../shared/bpf/flow_defs.h"
#include "../../../../shared/bpf/flow_event.h"
#include "../../../../shared/bpf/flow_maps.h"

/*
 * Application delay: the gap between this socket sending a request and the
 * first response byte arriving on it.
 *
 * This reads socket timing only. No payload is touched, so it works for TLS
 * exactly as it does for plaintext. What it measures is time-to-first-response-
 * byte on a socket, which equals request latency for a request/response
 * exchange but not for pipelined or multiplexed protocols where several
 * requests share one connection.
 */

#define APP_DELAY_MAX_US 60000000ULL

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 32768);
    __type(key, __u64);
    __type(value, __u64);
} app_delay_start SEC(".maps");

/* Called when the socket sends. Records the start of an outstanding request. */
static __always_inline void mark_request_sent(struct sock *sk)
{
    if (!sk) {
        return;
    }

    __u64 key = (__u64)sk;
    // Keep the earliest unanswered send so the delay covers the whole wait.
    if (bpf_map_lookup_elem(&app_delay_start, &key)) {
        return;
    }

    __u64 now = bpf_ktime_get_ns();
    bpf_map_update_elem(&app_delay_start, &key, &now, BPF_ANY);
}

/*
 * Called when the socket receives. Returns the elapsed microseconds since the
 * matching send, or 0 when this socket has no outstanding request.
 */
static __always_inline __u32 take_response_delay_us(struct sock *sk)
{
    if (!sk) {
        return 0;
    }

    __u64 key = (__u64)sk;
    __u64 *started = bpf_map_lookup_elem(&app_delay_start, &key);
    if (!started) {
        return 0;
    }

    __u64 now = bpf_ktime_get_ns();
    __u64 elapsed_ns = now - *started;
    bpf_map_delete_elem(&app_delay_start, &key);

    if (now < *started) {
        return 0;
    }

    __u64 elapsed_us = elapsed_ns / 1000;
    // A long-idle keep-alive socket would otherwise report its idle time as
    // application latency.
    if (elapsed_us == 0 || elapsed_us > APP_DELAY_MAX_US) {
        return 0;
    }
    return (__u32)elapsed_us;
}

#endif // APPLICATION_DELAY_H
