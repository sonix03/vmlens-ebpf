#ifndef METRICS_RTT_H
#define METRICS_RTT_H

/*
 * RTT for the current product topology is measured by the VMLens
 * connectivity probe, not by packet payload inspection.
 *
 * Keep this file as the explicit home for future eBPF RTT work:
 * SYN/SYN-ACK timing, TCP_INFO sampling, or socket-level latency.
 */

#define RTT_SOURCE_CONNECTIVITY_PROBE 1

#endif // METRICS_RTT_H
