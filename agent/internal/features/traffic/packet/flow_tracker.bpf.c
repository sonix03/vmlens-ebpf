//go:build ignore

// SPDX-License-Identifier: GPL-2.0
// Flow tracker. The only payload it reads is an HTTP/1.x status line, from
// which it keeps nothing but the status integer. It never reads user buffers.
#include "vmlinux.h"
#include <bpf/bpf_core_read.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

#include "../../../shared/bpf/flow_defs.h"
#include "../../../shared/bpf/flow_event.h"
#include "../../../shared/bpf/flow_maps.h"

/*
 * Single translation unit for the current loader.
 *
 * The actual feature code lives next to its domain:
 * - traffic/packet: TC packet ingress/egress capture
 * - protocols/transport/tcp/connection: socket/kprobe connection capture
 * - protocols/transport/tcp/rtt: TCP srtt_us sampling
 * - protocols/transport/tcp/retrans: tcp_retransmit_skb counting
 * - protocols/application/delay: request/response timing on the socket
 */
#include "../../protocols/transport/tcp/connection/connect.bpf.c"
#include "../../protocols/transport/tcp/rtt/rtt.bpf.c"
#include "../../protocols/transport/tcp/retrans/retrans.bpf.c"
#include "../../protocols/application/delay/delay.bpf.c"
#include "tc_ingress.bpf.c"
#include "tc_egress.bpf.c"

char LICENSE[] SEC("license") = "GPL";
