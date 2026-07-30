//go:build ignore

#ifndef TRANSPORT_TCP_RETRANS_BPF_C
#define TRANSPORT_TCP_RETRANS_BPF_C

#include "retransmit.bpf.h"

SEC("kprobe/tcp_retransmit_skb")
int BPF_KPROBE(trace_tcp_retransmit, struct sock *sk)
{
    return emit_tcp_retransmission(sk);
}

#endif // TRANSPORT_TCP_RETRANS_BPF_C
