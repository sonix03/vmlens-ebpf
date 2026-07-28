#ifndef METRICS_PORTS_H
#define METRICS_PORTS_H

#include "../common/flow_defs.h"
#include "../common/flow_event.h"

static __always_inline void set_ports(struct flow_event *event, __u16 src_port, __u16 dst_port)
{
    event->src_port = src_port;
    event->dst_port = dst_port;
}

static __always_inline void set_directional_ports(struct flow_event *event, __u8 direction,
                                                  __u16 src_port, __u16 dst_port)
{
    if (direction == DIR_INGRESS) {
        set_ports(event, dst_port, src_port);
        return;
    }
    set_ports(event, src_port, dst_port);
}

#endif // METRICS_PORTS_H
