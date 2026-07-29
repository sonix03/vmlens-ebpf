#ifndef FLOW_EVENT_H
#define FLOW_EVENT_H

struct flow_event {
    __u64 timestamp_ns;
    __u64 bytes;
    __u8 src_addr[16];
    __u8 dst_addr[16];
    __u32 connections;
    __u16 src_port;
    __u16 dst_port;
    __u16 family;
    __u8 protocol;
    __u8 direction;
    __u32 packets;
    __u32 error_count;
} __attribute__((packed));

#endif // FLOW_EVENT_H
