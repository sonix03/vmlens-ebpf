#ifndef APPLICATION_HTTP_STATUS_H
#define APPLICATION_HTTP_STATUS_H

#include "../../../../shared/bpf/flow_defs.h"
#include "../../../../shared/bpf/flow_event.h"
#include "../../../traffic/packet/packet_reader.bpf.h"

/*
 * Derives an HTTP response status code from the start of a TCP payload.
 *
 * Privacy boundary: the payload is read inside the kernel and only the three
 * status digits ever leave this function, as an integer. No payload bytes are
 * copied into the ring buffer, so nothing reaches userspace that could carry
 * a header value, a cookie, or a body. Encrypted traffic yields nothing here
 * by construction, because the plaintext never appears on the wire.
 */

#define TCP_DATA_OFFSET_FIELD 12
#define HTTP_STATUS_PREFIX_LEN 9
#define HTTP_STATUS_DIGITS 3

static __always_inline __u64 tcp_header_length(void *data, void *data_end, __u64 l4_offset)
{
    __u8 data_offset = 0;

    if (!read_packet_bytes(data, data_end, l4_offset + TCP_DATA_OFFSET_FIELD,
                           &data_offset, sizeof(data_offset))) {
        return 0;
    }

    __u64 length = (__u64)(data_offset >> 4) * 4;
    if (length < 20 || length > 60) {
        return 0;
    }
    return length;
}

static __always_inline int is_http_response_prefix(const __u8 prefix[HTTP_STATUS_PREFIX_LEN])
{
    // "HTTP/1." then a minor version digit then a space.
    return prefix[0] == 'H' && prefix[1] == 'T' && prefix[2] == 'T' && prefix[3] == 'P' &&
           prefix[4] == '/' && prefix[5] == '1' && prefix[6] == '.' &&
           prefix[7] >= '0' && prefix[7] <= '9' && prefix[8] == ' ';
}

/*
 * Returns the status code, or 0 when this packet does not start an HTTP/1.x
 * response. Only the first packet of a response carries the status line, so a
 * continuation or a request simply reads as 0.
 */
static __always_inline __u16 http_response_status(void *data, void *data_end,
                                                  __u64 l4_offset, __u8 protocol)
{
    if (protocol != IPPROTO_TCP_VALUE) {
        return 0;
    }

    __u64 header_length = tcp_header_length(data, data_end, l4_offset);
    if (header_length == 0) {
        return 0;
    }

    __u64 payload_offset = l4_offset + header_length;
    __u8 prefix[HTTP_STATUS_PREFIX_LEN] = {};
    if (!read_packet_bytes(data, data_end, payload_offset, prefix, sizeof(prefix))) {
        return 0;
    }
    if (!is_http_response_prefix(prefix)) {
        return 0;
    }

    __u8 digits[HTTP_STATUS_DIGITS] = {};
    if (!read_packet_bytes(data, data_end, payload_offset + HTTP_STATUS_PREFIX_LEN,
                           digits, sizeof(digits))) {
        return 0;
    }

    __u16 status = 0;
#pragma unroll
    for (int index = 0; index < HTTP_STATUS_DIGITS; index++) {
        if (digits[index] < '0' || digits[index] > '9') {
            return 0;
        }
        status = status * 10 + (digits[index] - '0');
    }

    if (status < 100 || status > 599) {
        return 0;
    }
    return status;
}

#endif // APPLICATION_HTTP_STATUS_H
