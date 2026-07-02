# Privacy boundary

VMLens collects SSH login/logout time, Linux username, remote address and port, authentication method name, `sshd` PID, process metadata, sanitized executable arguments, resource counters, and TCP endpoint metadata.

It does not collect authentication secrets, private keys, keystrokes, terminal contents, file contents, packet payloads, TLS plaintext, or screen recordings. The code contains no remote upload implementation and the metrics listener is loopback-only by default.

The sanitizer redacts values following secret-like flags, attached `-pVALUE`, sensitive environment assignments such as `TOKEN=value`, and authorization/cookie-style headers. Sanitization is heuristic; administrators should avoid putting secrets on command lines at all because they can also be exposed by standard Linux process inspection. The implementation favors over-redaction.

JSONL is local under `/var/log/vmlens` with restrictive permissions. Remote and destination IP logging can be disabled. Log rotation/retention must be integrated with the host's `logrotate` or journaling policy; `retention_days` declares policy intent but v0.1 does not delete files automatically.
