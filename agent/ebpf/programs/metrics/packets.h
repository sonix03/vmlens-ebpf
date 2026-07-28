#ifndef METRICS_PACKETS_H
#define METRICS_PACKETS_H

#include "../common/flow_event.h"

static __always_inline void count_packet(struct flow_event *event)
{
    event->packets = 1;
}

#endif // METRICS_PACKETS_H
