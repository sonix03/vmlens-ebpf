#ifndef VMLENS_FLOW_KEY_H
#define VMLENS_FLOW_KEY_H

struct vmlens_flow_key {
    __u8 src_addr[16];
    __u8 dst_addr[16];
    __u16 src_port;
    __u16 dst_port;
    __u8 protocol;
    __u16 family;
};

#endif
