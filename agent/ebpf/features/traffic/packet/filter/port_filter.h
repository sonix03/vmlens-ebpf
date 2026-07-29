#ifndef PORT_FILTER_H
#define PORT_FILTER_H

#include "../../../../shared/bpf/flow_defs.h"
#include "../../../../shared/bpf/flow_maps.h"

static __always_inline int ignored_port(__u16 port)
{
    if (port == 0) {
        return 0;
    }

    __u32 key = port;
    return bpf_map_lookup_elem(&ignored_ports, &key) != 0;
}

static __always_inline int ignored_transport_ports(__u8 protocol, __u16 src_port, __u16 dst_port)
{
    if (protocol != IPPROTO_TCP_VALUE && protocol != IPPROTO_UDP_VALUE) {
        return 0;
    }

    return ignored_port(src_port) || ignored_port(dst_port);
}

#endif // PORT_FILTER_H
