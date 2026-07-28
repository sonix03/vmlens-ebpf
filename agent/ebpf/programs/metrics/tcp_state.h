#ifndef METRICS_TCP_STATE_H
#define METRICS_TCP_STATE_H

#include "../common/flow_defs.h"

#define TCP_FLAG_FIN 0x01
#define TCP_FLAG_SYN 0x02
#define TCP_FLAG_RST 0x04
#define TCP_FLAG_ACK 0x10

static __always_inline int tcp_is_connect_attempt(__u8 protocol, __u8 tcp_flags)
{
    return protocol == IPPROTO_TCP_VALUE && (tcp_flags & TCP_FLAG_SYN) && !(tcp_flags & TCP_FLAG_ACK);
}

static __always_inline int tcp_is_reset(__u8 protocol, __u8 tcp_flags)
{
    return protocol == IPPROTO_TCP_VALUE && (tcp_flags & TCP_FLAG_RST);
}

#endif // METRICS_TCP_STATE_H
