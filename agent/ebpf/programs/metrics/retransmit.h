#ifndef METRICS_RETRANSMIT_H
#define METRICS_RETRANSMIT_H

/*
 * Retransmission is intentionally separated from generic error_count.
 * Current TC capture records TCP RST as failed-attempt/error signal.
 * Future implementation can add TCP sequence tracking here.
 */

#define RETRANSMIT_NOT_TRACKED_YET 1

#endif // METRICS_RETRANSMIT_H
