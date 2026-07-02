//go:build ignore

// SPDX-License-Identifier: GPL-2.0
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include "types.h"

#define IPPROTO_TCP_VALUE 6

struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 1 << 24);
} events SEC(".maps");

/*
 * tcp_sendmsg/tcp_recvmsg return the actual number of bytes from a kretprobe.
 * We retain metadata between entry and return, keyed by PID/TID. Payload bytes
 * are never copied.
 */
struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 32768);
    __type(key, __u64);
    __type(value, struct vmlens_event);
} pending_send SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 32768);
    __type(key, __u64);
    __type(value, struct vmlens_event);
} pending_receive SEC(".maps");

static __always_inline void fill_process(struct vmlens_event *e)
{
    __u64 id = bpf_get_current_pid_tgid();
    __u64 ug = bpf_get_current_uid_gid();
    struct task_struct *task = (struct task_struct *)bpf_get_current_task();

    e->pid = id >> 32;
    e->uid = (__u32)ug;
    e->gid = ug >> 32;
    e->ppid = BPF_CORE_READ(task, real_parent, tgid);
    e->timestamp_ns = bpf_ktime_get_ns();
    bpf_get_current_comm(e->comm, sizeof(e->comm));
}

static __always_inline void fill_socket(struct vmlens_event *e, struct sock *sk,
                                        __u8 direction)
{
    __u32 local = BPF_CORE_READ(sk, __sk_common.skc_rcv_saddr);
    __u32 remote = BPF_CORE_READ(sk, __sk_common.skc_daddr);
    __u16 local_port = BPF_CORE_READ(sk, __sk_common.skc_num);
    __u16 remote_port = __builtin_bswap16(
        BPF_CORE_READ(sk, __sk_common.skc_dport));

    e->family = BPF_CORE_READ(sk, __sk_common.skc_family);
    e->direction = direction;
    e->protocol = IPPROTO_TCP_VALUE;
    if (direction == VMLENS_DIRECTION_EGRESS) {
        e->src_port = local_port;
        e->dst_port = remote_port;
        __builtin_memcpy(e->src_addr, &local, sizeof(local));
        __builtin_memcpy(e->dst_addr, &remote, sizeof(remote));
    } else {
        e->src_port = remote_port;
        e->dst_port = local_port;
        __builtin_memcpy(e->src_addr, &remote, sizeof(remote));
        __builtin_memcpy(e->dst_addr, &local, sizeof(local));
    }
}

static __always_inline int emit_connection(struct sock *sk, __u32 type,
                                           __u8 direction)
{
    struct vmlens_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;
    __builtin_memset(e, 0, sizeof(*e));
    e->type = type;
    e->connection_count = 1;
    fill_process(e);
    fill_socket(e, sk, direction);
    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("tracepoint/syscalls/sys_enter_execve")
int trace_exec(struct trace_event_raw_sys_enter *ctx)
{
    struct vmlens_event *e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0;
    __builtin_memset(e, 0, sizeof(*e));
    e->type = VMLENS_EVENT_EXEC;
    fill_process(e);
    bpf_probe_read_user_str(e->filename, sizeof(e->filename),
                            (const void *)ctx->args[0]);
    bpf_ringbuf_submit(e, 0);
    return 0;
}

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(trace_tcp_connect, struct sock *sk)
{
    return emit_connection(sk, VMLENS_EVENT_CONNECT,
                           VMLENS_DIRECTION_EGRESS);
}

SEC("kretprobe/inet_csk_accept")
int trace_tcp_accept(struct pt_regs *ctx)
{
    struct sock *sk = (struct sock *)PT_REGS_RC(ctx);
    if (!sk)
        return 0;
    return emit_connection(sk, VMLENS_EVENT_ACCEPT,
                           VMLENS_DIRECTION_INGRESS);
}

static __always_inline int remember_io(void *map, struct sock *sk, __u32 type,
                                       __u8 direction)
{
    __u64 key = bpf_get_current_pid_tgid();
    struct vmlens_event e = {};
    e.type = type;
    fill_process(&e);
    fill_socket(&e, sk, direction);
    return bpf_map_update_elem(map, &key, &e, BPF_ANY);
}

static __always_inline int finish_io(void *map, struct pt_regs *ctx)
{
    __u64 key = bpf_get_current_pid_tgid();
    struct vmlens_event *saved = bpf_map_lookup_elem(map, &key);
    /* Kernel signature returns int; upper register bits may be stale. */
    int result = (int)PT_REGS_RC(ctx);
    if (!saved)
        return 0;
    if (result > 0) {
        struct vmlens_event *e =
            bpf_ringbuf_reserve(&events, sizeof(*e), 0);
        if (e) {
            __builtin_memcpy(e, saved, sizeof(*e));
            e->bytes = (__u64)result;
            /*
             * tcp_sendmsg/tcp_recvmsg account bytes, not wire packets.
             * packets remains zero rather than publishing a misleading value.
             */
            bpf_ringbuf_submit(e, 0);
        }
    }
    bpf_map_delete_elem(map, &key);
    return 0;
}

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_sendmsg, struct sock *sk)
{
    return remember_io(&pending_send, sk, VMLENS_EVENT_SEND,
                       VMLENS_DIRECTION_EGRESS);
}

SEC("kretprobe/tcp_sendmsg")
int trace_tcp_sendmsg_return(struct pt_regs *ctx)
{
    return finish_io(&pending_send, ctx);
}

SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_tcp_recvmsg, struct sock *sk)
{
    return remember_io(&pending_receive, sk, VMLENS_EVENT_RECEIVE,
                       VMLENS_DIRECTION_INGRESS);
}

SEC("kretprobe/tcp_recvmsg")
int trace_tcp_recvmsg_return(struct pt_regs *ctx)
{
    return finish_io(&pending_receive, ctx);
}

char LICENSE[] SEC("license") = "GPL";
