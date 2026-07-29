#ifndef VMLENS_EVENT_HEADER_H
#define VMLENS_EVENT_HEADER_H

struct vmlens_event_header {
    __u16 type;
    __u16 size;
    __u64 timestamp_ns;
};

#endif
