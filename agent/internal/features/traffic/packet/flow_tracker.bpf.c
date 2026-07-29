//go:build ignore

// SPDX-License-Identifier: GPL-2.0
// Metadata-only flow tracker. It never reads packet payloads or user buffers.
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
 */
#include "../../protocols/transport/tcp/connection/connect.bpf.c"
#include "tc_ingress.bpf.c"
#include "tc_egress.bpf.c"

char LICENSE[] SEC("license") = "GPL";
