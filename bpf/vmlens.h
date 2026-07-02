#ifndef __VMLENS_H
#define __VMLENS_H

#define VMLENS_COMM_LEN 16
#define VMLENS_PATH_LEN 256

enum vmlens_event_type { VMLENS_EXEC = 1, VMLENS_EXIT = 2, VMLENS_TCP_CONNECT = 3, VMLENS_SCHED = 4 };

struct vmlens_event {
    __u64 timestamp_ns;
    __u32 type;
    __u32 pid;
    __u32 ppid;
    __u32 uid;
    __u32 gid;
    __u16 family;
    __u16 dst_port;
    __u8 dst_addr[16];
    char comm[VMLENS_COMM_LEN];
    char filename[VMLENS_PATH_LEN];
};

#endif
