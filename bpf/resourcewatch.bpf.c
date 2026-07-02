// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include "vmlens.h"

struct cpu_key { __u32 pid; };

struct { 
    __uint(type, BPF_MAP_TYPE_LRU_HASH); 
    __uint(max_entries, 32768); 
    __type(key, struct cpu_key); __type(value, __u64); 
} context_switches SEC(".maps");

// Lightweight scheduler activity signal. Accurate CPU/RSS/I/O accounting for
// v0.1 is sampled from /proc by userspace and correlated using the same PID.
SEC("tracepoint/sched/sched_switch")

int count_switch(void *ctx) {
    struct cpu_key key = { 
        .pid = bpf_get_current_pid_tgid() >> 32 
    };
    __u64 one = 1, *value = bpf_map_lookup_elem(&context_switches, &key);
    if (value)
        __sync_fetch_and_add(value, 1); 
    else 
        bpf_map_update_elem(&context_switches, &key, &one, BPF_ANY);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
