// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include "vmlens.h"

struct { __uint(type, BPF_MAP_TYPE_RINGBUF); 
    __uint(max_entries, 1 << 24); 
} events SEC(".maps");

SEC("tracepoint/syscalls/sys_enter_execve")
int trace_exec(struct trace_event_raw_sys_enter *ctx) {
    struct vmlens_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;
    __u64 id = bpf_get_current_pid_tgid();
    __u64 ug = bpf_get_current_uid_gid();
    e->type = VMLENS_EXEC; 
    e->pid = id >> 32; 
    e->uid = ug; 
    e->gid = ug >> 32;
    e->timestamp_ns = bpf_ktime_get_ns();
    bpf_get_current_comm(e->comm, sizeof(e->comm));
    bpf_probe_read_user_str(e->filename, sizeof(e->filename), (const void *)ctx->args[0]);
    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("tracepoint/sched/sched_process_exit")
int trace_exit(void *ctx) {
    struct vmlens_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e) return 0;
    e->type = VMLENS_EXIT; e->pid = bpf_get_current_pid_tgid() >> 32;
    e->timestamp_ns = bpf_ktime_get_ns(); bpf_get_current_comm(e->comm, sizeof(e->comm));
    bpf_ringbuf_submit(e, 0); return 0;
}
char LICENSE[] SEC("license") = "GPL";
