#ifndef __VMLENS_TYPES_H
#define __VMLENS_TYPES_H

#define VMLENS_COMM_LEN 16
#define VMLENS_PATH_LEN 256

enum vmlens_event_type {
    VMLENS_EVENT_EXEC = 1,
    VMLENS_EVENT_CONNECT = 2,
    VMLENS_EVENT_ACCEPT = 3,
    VMLENS_EVENT_SEND = 4,
    VMLENS_EVENT_RECEIVE = 5,
};

enum vmlens_direction {
    VMLENS_DIRECTION_INGRESS = 1,
    VMLENS_DIRECTION_EGRESS = 2,
};

/*
 * This is metadata only. There is intentionally no packet or payload buffer.
 * Keep the field order synchronized with rawEvent in loader.go.
 */
struct vmlens_event {
    __u64 timestamp_ns;
    __u64 bytes;
    __u32 type;
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u32 gid;
    __u32 packets;
    __u32 connection_count;
    __u16 family;
    __u16 src_port;
    __u16 dst_port;
    __u8 direction;
    __u8 protocol;
    __u8 src_addr[16];
    __u8 dst_addr[16];
    char comm[VMLENS_COMM_LEN];
    char filename[VMLENS_PATH_LEN];
};

#endif
