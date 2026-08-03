#ifndef FLOW_MAPS_H
#define FLOW_MAPS_H

#include "flow_event.h"

enum capture_stat_key {
    CAPTURE_STAT_SEEN = 0,
    CAPTURE_STAT_EMITTED = 1,
    CAPTURE_STAT_ETH_READ_FAIL = 2,
    CAPTURE_STAT_ETH_UNSUPPORTED = 3,
    CAPTURE_STAT_IPV4_READ_FAIL = 4,
    CAPTURE_STAT_IPV4_INVALID = 5,
    CAPTURE_STAT_IPV6_READ_FAIL = 6,
    CAPTURE_STAT_IP_PROTOCOL_UNSUPPORTED = 7,
    CAPTURE_STAT_TRANSPORT_READ_FAIL = 8,
    CAPTURE_STAT_TCP_READ_FAIL = 9,
    CAPTURE_STAT_RINGBUF_FULL = 10,
    CAPTURE_STAT_PORT_IGNORED = 11,
    CAPTURE_STAT_MAX = 12,
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

/*
 * The socket behind an in-flight sendmsg/recvmsg, keyed by pid_tgid alongside
 * pending_io. The return probe needs it to settle application delay at the
 * moment data actually arrived rather than when the call was entered.
 */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 32768);
    __type(key, __u64);
    __type(value, __u64);
} pending_sock SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, CAPTURE_STAT_MAX);
    __type(key, __u32);
    __type(value, __u64);
} capture_stats SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 256);
    __type(key, __u32);
    __type(value, __u8);
} ignored_ports SEC(".maps");

static __always_inline void count_capture_stat(__u32 key)
{
    __u64 *counter = bpf_map_lookup_elem(&capture_stats, &key);

    if (counter) {
        *counter += 1;
    }
}

#endif // FLOW_MAPS_H
