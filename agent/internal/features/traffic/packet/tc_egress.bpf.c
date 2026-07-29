//go:build ignore

#ifndef TRAFFIC_PACKET_TC_EGRESS_BPF_C
#define TRAFFIC_PACKET_TC_EGRESS_BPF_C

#include "parser.bpf.h"

SEC("tcx/egress")
int tc_egress(struct __sk_buff *skb)
{
    return emit_tc_packet(skb, DIR_EGRESS);
}

#endif // TRAFFIC_PACKET_TC_EGRESS_BPF_C
