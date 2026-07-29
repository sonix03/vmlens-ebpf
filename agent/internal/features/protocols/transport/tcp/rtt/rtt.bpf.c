//go:build ignore

#ifndef TRANSPORT_TCP_RTT_BPF_C
#define TRANSPORT_TCP_RTT_BPF_C

#include "rtt.bpf.h"

/*
 * Active RTT today is produced by the connectivity probe and reduced in
 * reducer.go. Kernel RTT tracking will be added here when TCP_INFO or
 * SYN/SYN-ACK timing is enabled.
 */

#endif // TRANSPORT_TCP_RTT_BPF_C
