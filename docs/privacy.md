# Privacy boundary

VMLens collects SSH login/logout time, Linux username, remote address and port, authentication method name, `sshd` PID, process metadata, sanitized executable arguments, resource counters, and TCP endpoint metadata.

It does not collect authentication secrets, private keys, keystrokes, terminal contents, file contents, TLS plaintext, or screen recordings. The code contains no remote upload implementation and the metrics listener is loopback-only by default.

## The one payload read, and its limits

There is exactly one place where VMLens looks at packet payload: the eBPF
program reads the first bytes of a TCP payload to decide whether they begin a
plaintext `HTTP/1.x` status line, and if so converts the three status digits
into an integer.

What this means in practice:

| | |
| --- | --- |
| What is read | The first 12 bytes of a TCP payload, in kernel space only. |
| What is kept | An integer status code, e.g. `404`. Nothing else. |
| What leaves the kernel | The integer. No payload bytes are copied into the ring buffer, so none can reach userspace, the database, or the API. |
| What is never read | Request lines, URLs, headers, cookies, tokens, reason phrases, and bodies. The read stops after the status digits. |
| Encrypted traffic | Yields nothing. The plaintext never appears on the wire, so HTTPS flows report no status at all. |

The implementation is `agent/internal/features/protocols/application/http/status.bpf.h`.
The read is bounds-checked against the packet end and returns 0 for anything
that is not an HTTP/1.x status line, which is the common case.

Application delay is derived separately from socket timing only
(`.../application/delay/delay.bpf.h`) and reads no payload at all, which is why
it works for encrypted traffic as well.

The sanitizer redacts values following secret-like flags, attached `-pVALUE`, sensitive environment assignments such as `TOKEN=value`, and authorization/cookie-style headers. Sanitization is heuristic; administrators should avoid putting secrets on command lines at all because they can also be exposed by standard Linux process inspection. The implementation favors over-redaction.

JSONL is local under `/var/log/vmlens` with restrictive permissions. Remote and destination IP logging can be disabled. Log rotation/retention must be integrated with the host's `logrotate` or journaling policy; `retention_days` declares policy intent but v0.1 does not delete files automatically.
