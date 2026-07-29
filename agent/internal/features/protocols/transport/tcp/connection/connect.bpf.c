//go:build ignore

#ifndef TRANSPORT_TCP_CONNECTION_CONNECT_BPF_C
#define TRANSPORT_TCP_CONNECTION_CONNECT_BPF_C

#include "socket_capture.bpf.h"

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(trace_tcp_connect, struct sock *sk)
{
    return emit_connection(sk, IPPROTO_TCP_VALUE, DIR_EGRESS);
}

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(trace_tcp_v6_connect, struct sock *sk)
{
    return emit_connection(sk, IPPROTO_TCP_VALUE, DIR_EGRESS);
}

SEC("kretprobe/inet_csk_accept")
int trace_tcp_accept(struct pt_regs *ctx)
{
    struct sock *sk = (struct sock *)PT_REGS_RC(ctx);
    return sk ? emit_connection(sk, IPPROTO_TCP_VALUE, DIR_INGRESS) : 0;
}

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_send, struct sock *sk)
{
    return remember_io(sk, IPPROTO_TCP_VALUE, DIR_EGRESS);
}

SEC("kretprobe/tcp_sendmsg")
int trace_tcp_send_ret(struct pt_regs *ctx)
{
    return finish_io(ctx);
}

SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_tcp_recv, struct sock *sk)
{
    return remember_io(sk, IPPROTO_TCP_VALUE, DIR_INGRESS);
}

SEC("kretprobe/tcp_recvmsg")
int trace_tcp_recv_ret(struct pt_regs *ctx)
{
    return finish_io(ctx);
}

SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(trace_udp_send, struct sock *sk)
{
    return remember_io(sk, IPPROTO_UDP_VALUE, DIR_EGRESS);
}

SEC("kretprobe/udp_sendmsg")
int trace_udp_send_ret(struct pt_regs *ctx)
{
    return finish_io(ctx);
}

SEC("kprobe/udp_recvmsg")
int BPF_KPROBE(trace_udp_recv, struct sock *sk)
{
    return remember_io(sk, IPPROTO_UDP_VALUE, DIR_INGRESS);
}

SEC("kretprobe/udp_recvmsg")
int trace_udp_recv_ret(struct pt_regs *ctx)
{
    return finish_io(ctx);
}

#endif // TRANSPORT_TCP_CONNECTION_CONNECT_BPF_C
