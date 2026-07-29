//go:build ignore

#ifndef TRAFFIC_PACKET_TC_INGRESS_BPF_C
#define TRAFFIC_PACKET_TC_INGRESS_BPF_C

#include "parser.bpf.h"

SEC("tcx/ingress")
int tc_ingress(struct __sk_buff *skb)
{
    return emit_tc_packet(skb, DIR_INGRESS);
}

#endif // TRAFFIC_PACKET_TC_INGRESS_BPF_C
