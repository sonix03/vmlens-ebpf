#ifndef TRANSPORT_PARSER_H
#define TRANSPORT_PARSER_H

#include "../../../shared/bpf/flow_defs.h"
#include "../../../shared/bpf/flow_maps.h"
#include "packet_reader.bpf.h"

struct transport_metadata {
    __u16 src_port;
    __u16 dst_port;
    __u8 tcp_flags;
};

static __always_inline int parse_transport_metadata(void *data, void *data_end,
                                                    __u64 l4_offset, __u8 protocol,
                                                    struct transport_metadata *metadata)
{
    if (protocol != IPPROTO_TCP_VALUE && protocol != IPPROTO_UDP_VALUE) {
        return 1;
    }

    if (!read_packet_bytes(data, data_end, l4_offset,
                           &metadata->src_port, sizeof(metadata->src_port)) ||
        !read_packet_bytes(data, data_end, l4_offset + sizeof(metadata->src_port),
                           &metadata->dst_port, sizeof(metadata->dst_port))) {
        count_capture_stat(CAPTURE_STAT_TRANSPORT_READ_FAIL);
        return 0;
    }

    metadata->src_port = bpf_ntohs(metadata->src_port);
    metadata->dst_port = bpf_ntohs(metadata->dst_port);

    if (protocol == IPPROTO_TCP_VALUE &&
        !read_packet_bytes(data, data_end, l4_offset + TCP_FLAGS_OFFSET,
                           &metadata->tcp_flags, sizeof(metadata->tcp_flags))) {
        count_capture_stat(CAPTURE_STAT_TCP_READ_FAIL);
        return 0;
    }

    return 1;
}

#endif // TRANSPORT_PARSER_H
