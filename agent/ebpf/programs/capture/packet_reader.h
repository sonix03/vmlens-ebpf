#ifndef PACKET_READER_H
#define PACKET_READER_H

static __always_inline int read_packet_bytes(void *data, void *data_end,
                                             __u64 offset, void *target, __u64 size)
{
    if (data + offset + size > data_end) {
        return 0;
    }

    __builtin_memcpy(target, data + offset, size);
    return 1;
}

#endif // PACKET_READER_H
