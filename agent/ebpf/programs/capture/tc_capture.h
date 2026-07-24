#ifndef VMLENS_TC_CAPTURE_H
#define VMLENS_TC_CAPTURE_H

#include "../common/flow_defs.h"
#include "../common/flow_event.h"

static __always_inline int load_bytes(void *data, void *data_end, __u64 offset, void *target, __u64 size)
{
    if (data + offset + size > data_end) return 0;
    __builtin_memcpy(target, data + offset, size);
    return 1;
}

static __always_inline int supported_ip_protocol(__u8 protocol)
{
    return protocol == IPPROTO_TCP_VALUE || protocol == IPPROTO_UDP_VALUE ||
           protocol == IPPROTO_ICMP_VALUE || protocol == IPPROTO_ICMPV6_VALUE;
}

static __always_inline void copy_v4(__u8 destination[16], __u32 value)
{
    __builtin_memset(destination, 0, 16);
    __builtin_memcpy(destination, &value, sizeof(value));
}

static __always_inline void fill_tc_addresses(struct flow_event *event, __u8 direction,
                                               __u8 src_addr[16], __u8 dst_addr[16],
                                               __u16 src_port, __u16 dst_port)
{
    if (direction == DIR_INGRESS) {
        __builtin_memcpy(event->src_addr, dst_addr, 16);
        __builtin_memcpy(event->dst_addr, src_addr, 16);
        event->src_port = dst_port;
        event->dst_port = src_port;
        return;
    }
    __builtin_memcpy(event->src_addr, src_addr, 16);
    __builtin_memcpy(event->dst_addr, dst_addr, 16);
    event->src_port = src_port;
    event->dst_port = dst_port;
}

static __always_inline int emit_tc_packet(struct __sk_buff *skb, __u8 direction)
{
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;
    __u16 eth_proto = 0;
    __u64 network_offset = ETH_HEADER_LEN;
    __u8 protocol = 0;
    __u16 family = 0;
    __u16 src_port = 0;
    __u16 dst_port = 0;
    __u8 src_addr[16] = {};
    __u8 dst_addr[16] = {};
    __u8 tcp_flags = 0;

    if (!load_bytes(data, data_end, ETH_PROTOCOL_OFFSET, &eth_proto, sizeof(eth_proto))) return TC_ACT_OK;
    eth_proto = bpf_ntohs(eth_proto);

    if (eth_proto == ETH_P_IP_VALUE) {
        __u8 version_ihl = 0;
        __u32 saddr = 0;
        __u32 daddr = 0;
        if (!load_bytes(data, data_end, network_offset, &version_ihl, sizeof(version_ihl))) return TC_ACT_OK;
        if ((version_ihl >> 4) != 4) return TC_ACT_OK;
        __u8 ihl = (version_ihl & 0x0f) * 4;
        if (ihl < IPV4_MIN_HEADER_LEN) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + IPV4_PROTOCOL_OFFSET, &protocol, sizeof(protocol))) return TC_ACT_OK;
        if (!supported_ip_protocol(protocol)) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + IPV4_SRC_OFFSET, &saddr, sizeof(saddr))) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + IPV4_DST_OFFSET, &daddr, sizeof(daddr))) return TC_ACT_OK;
        copy_v4(src_addr, saddr);
        copy_v4(dst_addr, daddr);
        network_offset += ihl;
        family = AF_INET_VALUE;
    } else if (eth_proto == ETH_P_IPV6_VALUE) {
        if (!load_bytes(data, data_end, network_offset + IPV6_NEXT_HEADER_OFFSET, &protocol, sizeof(protocol))) return TC_ACT_OK;
        if (!supported_ip_protocol(protocol)) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + IPV6_SRC_OFFSET, src_addr, 16)) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + IPV6_DST_OFFSET, dst_addr, 16)) return TC_ACT_OK;
        network_offset += IPV6_HEADER_LEN;
        family = AF_INET6_VALUE;
    } else {
        return TC_ACT_OK;
    }

    if (protocol == IPPROTO_TCP_VALUE || protocol == IPPROTO_UDP_VALUE) {
        if (!load_bytes(data, data_end, network_offset, &src_port, sizeof(src_port))) return TC_ACT_OK;
        if (!load_bytes(data, data_end, network_offset + 2, &dst_port, sizeof(dst_port))) return TC_ACT_OK;
        src_port = bpf_ntohs(src_port);
        dst_port = bpf_ntohs(dst_port);
        if (protocol == IPPROTO_TCP_VALUE) {
            if (!load_bytes(data, data_end, network_offset + TCP_FLAGS_OFFSET, &tcp_flags, sizeof(tcp_flags))) return TC_ACT_OK;
        }
    }

    struct flow_event *event = bpf_ringbuf_reserve(&events, sizeof(*event), 0);
    if (!event) return TC_ACT_OK;
    __builtin_memset(event, 0, sizeof(*event));
    event->timestamp_ns = bpf_ktime_get_ns();
    event->bytes = skb->len;
    event->family = family;
    event->protocol = protocol;
    event->direction = direction;
    event->packets = 1;
    event->connections = (protocol == IPPROTO_TCP_VALUE && (tcp_flags & TCP_SYN) && !(tcp_flags & TCP_ACK)) ? 1 : 0;
    event->error_count = (protocol == IPPROTO_TCP_VALUE && (tcp_flags & TCP_RST)) ? 1 : 0;
    fill_tc_addresses(event, direction, src_addr, dst_addr, src_port, dst_port);
    bpf_ringbuf_submit(event, 0);
    return TC_ACT_OK;
}

#endif // VMLENS_TC_CAPTURE_H
