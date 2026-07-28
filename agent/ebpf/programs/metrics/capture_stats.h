#ifndef CAPTURE_STATS_H
#define CAPTURE_STATS_H

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
    CAPTURE_STAT_MAX = 11,
};

struct {
    __uint(type, BPF_MAP_TYPE_PERCPU_ARRAY);
    __uint(max_entries, CAPTURE_STAT_MAX);
    __type(key, __u32);
    __type(value, __u64);
} capture_stats SEC(".maps");

static __always_inline void count_capture_stat(__u32 key)
{
    __u64 *counter = bpf_map_lookup_elem(&capture_stats, &key);

    if (counter) {
        *counter += 1;
    }
}

#endif // CAPTURE_STATS_H
