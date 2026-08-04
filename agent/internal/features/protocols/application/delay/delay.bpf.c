//go:build ignore

#ifndef APPLICATION_DELAY_BPF_C
#define APPLICATION_DELAY_BPF_C

#include "../../transport/tcp/connection/socket_capture.bpf.h"
#include "delay.bpf.h"

/*
 * Application delay runs on its own probes rather than inside the connection
 * capture path, because that path is only attached in kprobe capture mode. In
 * TC mode the agent takes bytes from the TC hook and attaches no socket probes,
 * so hooking the delay there would have measured nothing in the mode the agent
 * normally runs in.
 *
 * These programs emit an event carrying only the delay. Byte and packet
 * accounting stays with whichever capture path is active, so nothing is counted
 * twice when both are attached.
 */

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_delay_send, struct sock *sk)
{
    mark_request_sent(sk);
    return 0;
}

SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_tcp_delay_recv, struct sock *sk)
{
    if (!sk) {
        return 0;
    }
    __u64 key = bpf_get_current_pid_tgid();
    __u64 sock_key = (__u64)sk;
    bpf_map_update_elem(&app_delay_recv_sock, &key, &sock_key, BPF_ANY);
    return 0;
}

SEC("kretprobe/tcp_recvmsg")
int trace_tcp_delay_recv_ret(struct pt_regs *ctx)
{
    __u64 key = bpf_get_current_pid_tgid();
    __u64 *sock_key = bpf_map_lookup_elem(&app_delay_recv_sock, &key);
    int result = (int)PT_REGS_RC(ctx);

    if (!sock_key) {
        return 0;
    }
    struct sock *sk = (struct sock *)*sock_key;
    bpf_map_delete_elem(&app_delay_recv_sock, &key);

    if (result <= 0) {
        return 0;
    }

    __u32 delay_us = take_response_delay_us(sk);
    if (delay_us == 0) {
        return 0;
    }

    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        count_capture_stat(CAPTURE_STAT_RINGBUF_FULL);
        return 0;
    }

    __builtin_memset(event, 0, sizeof(*event));
    // Ingress: this is the response arriving on the local socket.
    socket_metadata(event, sk, IPPROTO_TCP_VALUE, DIR_INGRESS);
    event->app_delay_us = delay_us;
    bpf_ringbuf_submit(event, 0);
    return 0;
}

#endif // APPLICATION_DELAY_BPF_C
