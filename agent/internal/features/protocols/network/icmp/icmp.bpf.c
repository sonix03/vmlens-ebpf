//go:build ignore

#ifndef NETWORK_ICMP_BPF_C
#define NETWORK_ICMP_BPF_C

/*
 * ICMP is currently collected from a packet socket in collector.go when TCX is
 * unavailable. This file is the explicit kernel-source location for future ICMP
 * TC/eBPF specialization.
 */

#endif // NETWORK_ICMP_BPF_C
