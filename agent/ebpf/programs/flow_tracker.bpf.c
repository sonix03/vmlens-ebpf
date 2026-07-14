//go:build ignore

// SPDX-License-Identifier: GPL-2.0
// Metadata-only flow tracker. It never reads packet payloads or user buffers.
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#include "common/flow_defs.h"
#include "common/flow_event.h"
#include "capture/socket_capture.h"
#include "capture/tc_capture.h"

SEC("kprobe/tcp_v4_connect")
int BPF_KPROBE(trace_tcp_connect, struct sock *sk) { return emit_connection(sk, IPPROTO_TCP_VALUE, DIR_EGRESS); }

SEC("kprobe/tcp_v6_connect")
int BPF_KPROBE(trace_tcp_v6_connect, struct sock *sk) { return emit_connection(sk, IPPROTO_TCP_VALUE, DIR_EGRESS); }

SEC("kretprobe/inet_csk_accept")
int trace_tcp_accept(struct pt_regs *ctx)
{
    struct sock *sk = (struct sock *)PT_REGS_RC(ctx);
    return sk ? emit_connection(sk, IPPROTO_TCP_VALUE, DIR_INGRESS) : 0;
}

SEC("kprobe/tcp_sendmsg")
int BPF_KPROBE(trace_tcp_send, struct sock *sk) { return remember_io(sk, IPPROTO_TCP_VALUE, DIR_EGRESS); }
SEC("kretprobe/tcp_sendmsg")
int trace_tcp_send_ret(struct pt_regs *ctx) { return finish_io(ctx); }
SEC("kprobe/tcp_recvmsg")
int BPF_KPROBE(trace_tcp_recv, struct sock *sk) { return remember_io(sk, IPPROTO_TCP_VALUE, DIR_INGRESS); }
SEC("kretprobe/tcp_recvmsg")
int trace_tcp_recv_ret(struct pt_regs *ctx) { return finish_io(ctx); }

SEC("kprobe/udp_sendmsg")
int BPF_KPROBE(trace_udp_send, struct sock *sk) { return remember_io(sk, IPPROTO_UDP_VALUE, DIR_EGRESS); }
SEC("kretprobe/udp_sendmsg")
int trace_udp_send_ret(struct pt_regs *ctx) { return finish_io(ctx); }
SEC("kprobe/udp_recvmsg")
int BPF_KPROBE(trace_udp_recv, struct sock *sk) { return remember_io(sk, IPPROTO_UDP_VALUE, DIR_INGRESS); }
SEC("kretprobe/udp_recvmsg")
int trace_udp_recv_ret(struct pt_regs *ctx) { return finish_io(ctx); }

SEC("tcx/ingress")
int tc_ingress(struct __sk_buff *skb) { return emit_tc_packet(skb, DIR_INGRESS); }

SEC("tcx/egress")
int tc_egress(struct __sk_buff *skb) { return emit_tc_packet(skb, DIR_EGRESS); }

char LICENSE[] SEC("license") = "GPL";
