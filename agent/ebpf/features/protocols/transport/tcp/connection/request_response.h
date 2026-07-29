#ifndef METRICS_REQUEST_RESPONSE_H
#define METRICS_REQUEST_RESPONSE_H

#include "../../../../../shared/bpf/flow_event.h"
#include "tcp_state.h"

static __always_inline void mark_connection(struct flow_event *event)
{
    event->connections = 1;
}

static __always_inline void mark_tc_request_response(struct flow_event *event,
                                                     __u8 protocol, __u8 tcp_flags)
{
    event->connections = tcp_is_connect_attempt(protocol, tcp_flags) ? 1 : 0;
    event->error_count = tcp_is_reset(protocol, tcp_flags) ? 1 : 0;
}

#endif // METRICS_REQUEST_RESPONSE_H
