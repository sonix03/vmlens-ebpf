//go:build ignore

#ifndef TRANSPORT_TCP_RTT_BPF_C
#define TRANSPORT_TCP_RTT_BPF_C

#include "rtt.bpf.h"

SEC("kprobe/tcp_rcv_established")
int BPF_KPROBE(trace_tcp_rtt, struct sock *sk)
{
    return emit_tcp_rtt_sample(sk);
}

#endif // TRANSPORT_TCP_RTT_BPF_C
