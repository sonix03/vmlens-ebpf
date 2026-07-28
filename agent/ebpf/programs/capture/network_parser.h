#ifndef NETWORK_PARSER_H
#define NETWORK_PARSER_H

#include "../common/flow_defs.h"
#include "../metrics/capture_stats.h"
#include "packet_reader.h"

struct network_metadata {
    __u64 l4_offset;
    __u8 src_addr[16];
    __u8 dst_addr[16];
    __u16 family;
    __u8 protocol;
};

static __always_inline int is_supported_ip_protocol(__u8 protocol)
{
    return protocol == IPPROTO_TCP_VALUE || protocol == IPPROTO_UDP_VALUE ||
           protocol == IPPROTO_ICMP_VALUE || protocol == IPPROTO_ICMPV6_VALUE;
}

static __always_inline void store_ipv4_address(__u8 destination[16], __u32 value)
{
    __builtin_memset(destination, 0, 16);
    __builtin_memcpy(destination, &value, sizeof(value));
}

static __always_inline int parse_ipv4_metadata(void *data, void *data_end,
                                               __u64 l3_offset,
                                               struct network_metadata *metadata)
{
    __u8 version_ihl = 0;
    __u32 saddr = 0;
    __u32 daddr = 0;

    if (!read_packet_bytes(data, data_end, l3_offset, &version_ihl, sizeof(version_ihl)) ||
        !read_packet_bytes(data, data_end, l3_offset + IPV4_PROTOCOL_OFFSET,
                           &metadata->protocol, sizeof(metadata->protocol))) {
        count_capture_stat(CAPTURE_STAT_IPV4_READ_FAIL);
        return 0;
    }

    if ((version_ihl >> 4) != 4) {
        count_capture_stat(CAPTURE_STAT_IPV4_INVALID);
        return 0;
    }

    __u8 ihl = (version_ihl & 0x0f) * 4;
    if (ihl < IPV4_MIN_HEADER_LEN) {
        count_capture_stat(CAPTURE_STAT_IPV4_INVALID);
        return 0;
    }

    if (!is_supported_ip_protocol(metadata->protocol)) {
        count_capture_stat(CAPTURE_STAT_IP_PROTOCOL_UNSUPPORTED);
        return 0;
    }

    if (!read_packet_bytes(data, data_end, l3_offset + IPV4_SRC_OFFSET, &saddr, sizeof(saddr)) ||
        !read_packet_bytes(data, data_end, l3_offset + IPV4_DST_OFFSET, &daddr, sizeof(daddr))) {
        count_capture_stat(CAPTURE_STAT_IPV4_READ_FAIL);
        return 0;
    }

    store_ipv4_address(metadata->src_addr, saddr);
    store_ipv4_address(metadata->dst_addr, daddr);
    metadata->family = AF_INET_VALUE;
    metadata->l4_offset = l3_offset + ihl;
    return 1;
}

static __always_inline int parse_ipv6_metadata(void *data, void *data_end,
                                               __u64 l3_offset,
                                               struct network_metadata *metadata)
{
    if (!read_packet_bytes(data, data_end, l3_offset + IPV6_NEXT_HEADER_OFFSET,
                           &metadata->protocol, sizeof(metadata->protocol))) {
        count_capture_stat(CAPTURE_STAT_IPV6_READ_FAIL);
        return 0;
    }

    if (!is_supported_ip_protocol(metadata->protocol)) {
        count_capture_stat(CAPTURE_STAT_IP_PROTOCOL_UNSUPPORTED);
        return 0;
    }

    if (!read_packet_bytes(data, data_end, l3_offset + IPV6_SRC_OFFSET, metadata->src_addr, 16) ||
        !read_packet_bytes(data, data_end, l3_offset + IPV6_DST_OFFSET, metadata->dst_addr, 16)) {
        count_capture_stat(CAPTURE_STAT_IPV6_READ_FAIL);
        return 0;
    }

    metadata->family = AF_INET6_VALUE;
    metadata->l4_offset = l3_offset + IPV6_HEADER_LEN;
    return 1;
}

static __always_inline int parse_network_metadata(void *data, void *data_end,
                                                  struct network_metadata *metadata)
{
    __u16 eth_proto = 0;
    __u64 l3_offset = ETH_HEADER_LEN;

    if (!read_packet_bytes(data, data_end, ETH_PROTOCOL_OFFSET, &eth_proto, sizeof(eth_proto))) {
        count_capture_stat(CAPTURE_STAT_ETH_READ_FAIL);
        return 0;
    }

    eth_proto = bpf_ntohs(eth_proto);
    if (eth_proto == ETH_P_IP_VALUE) {
        return parse_ipv4_metadata(data, data_end, l3_offset, metadata);
    }
    if (eth_proto == ETH_P_IPV6_VALUE) {
        return parse_ipv6_metadata(data, data_end, l3_offset, metadata);
    }

    count_capture_stat(CAPTURE_STAT_ETH_UNSUPPORTED);
    return 0;
}

#endif // NETWORK_PARSER_H
