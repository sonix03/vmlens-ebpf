#ifndef TC_CAPTURE_H
#define TC_CAPTURE_H

#include "../../../shared/bpf/flow_defs.h"
#include "../../../shared/bpf/flow_event.h"
#include "../../../shared/bpf/flow_maps.h"
#include "../../classification/ports.bpf.h"
#include "../../protocols/transport/tcp/connection/request_response.bpf.h"
#include "../bytes/bytes.bpf.h"
#include "../packets/packets.bpf.h"
#include "filter/port_filter.bpf.h"
#include "network_parser.bpf.h"
#include "transport_parser.bpf.h"

static __always_inline void fill_directional_tuple(struct flow_event *event, __u8 direction,
                                                   __u8 src_addr[16], __u8 dst_addr[16],
                                                   __u16 src_port, __u16 dst_port)
{
    if (direction == DIR_INGRESS) {
        __builtin_memcpy(event->src_addr, dst_addr, 16);
        __builtin_memcpy(event->dst_addr, src_addr, 16);
        set_directional_ports(event, direction, src_port, dst_port);
        return;
    }

    __builtin_memcpy(event->src_addr, src_addr, 16);
    __builtin_memcpy(event->dst_addr, dst_addr, 16);
    set_directional_ports(event, direction, src_port, dst_port);
}

static __always_inline void populate_tc_event(struct flow_event *event, struct __sk_buff *skb,
                                              __u8 direction,
                                              struct network_metadata *network,
                                              struct transport_metadata *transport)
{
    __builtin_memset(event, 0, sizeof(*event));
    event->timestamp_ns = bpf_ktime_get_ns();
    set_bytes(event, skb->len);
    event->family = network->family;
    event->protocol = network->protocol;
    event->direction = direction;
    count_packet(event);
    mark_tc_request_response(event, network->protocol, transport->tcp_flags);
    fill_directional_tuple(event, direction, network->src_addr, network->dst_addr,
                           transport->src_port, transport->dst_port);
}

static __always_inline int emit_tc_packet(struct __sk_buff *skb, __u8 direction)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    struct network_metadata network = {};
    struct transport_metadata transport = {};

    count_capture_stat(CAPTURE_STAT_SEEN);
    if (!parse_network_metadata(data, data_end, &network)) {
        return TC_ACT_OK;
    }
    if (!parse_transport_metadata(data, data_end, network.l4_offset, network.protocol, &transport)) {
        return TC_ACT_OK;
    }
    if (ignored_transport_ports(network.protocol, transport.src_port, transport.dst_port)) {
        count_capture_stat(CAPTURE_STAT_PORT_IGNORED);
        return TC_ACT_OK;
    }

    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) {
        count_capture_stat(CAPTURE_STAT_RINGBUF_FULL);
        return TC_ACT_OK;
    }

    populate_tc_event(event, skb, direction, &network, &transport);
    bpf_ringbuf_submit(event, 0);
    count_capture_stat(CAPTURE_STAT_EMITTED);
    return TC_ACT_OK;
}

#endif // TC_CAPTURE_H
