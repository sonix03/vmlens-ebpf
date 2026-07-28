#ifndef METRICS_BYTES_H
#define METRICS_BYTES_H

#include "../common/flow_event.h"

static __always_inline void set_bytes(struct flow_event *event, __u64 bytes)
{
    event->bytes = bytes;
}

#endif // METRICS_BYTES_H
