// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include "vmlens.h"

struct { __uint(type, BPF_MAP_TYPE_RINGBUF); __uint(max_entries, 1 << 22); } net_events SEC(".maps");

// This captures connect metadata only. It never reads skb payload data.
SEC("kprobe/tcp_v4_connect")

int BPF_KPROBE(tcp_v4_connect, struct sock *sk) {
    struct vmlens_event *e = bpf_ringbuf_reserve(&net_events, sizeof(*e), 0);
    if (!e) 
        return 0;
    e->type = VMLENS_TCP_CONNECT; 
    e->pid = bpf_get_current_pid_tgid() >> 32;
    e->timestamp_ns = bpf_ktime_get_ns(); 
    e->family = BPF_CORE_READ(sk, __sk_common.skc_family);
    e->dst_port = __builtin_bswap16(BPF_CORE_READ(sk, __sk_common.skc_dport));
    __u32 addr = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    __builtin_memcpy(e->dst_addr, &addr, sizeof(addr)); 
    bpf_get_current_comm(e->comm, sizeof(e->comm));
    bpf_ringbuf_submit(e, 0); 
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
