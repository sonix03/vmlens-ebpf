# SSH session analysis

VMLens creates a session when an `Accepted ... for USER from IP port PORT` record is observed. The `sshd[PID]` value becomes the correlation root. Each observed process records PID and PPID; walking that tree attaches the process, resource samples, and socket metadata to the session.

Time, TTY, and UID are supporting evidence but UID alone is not used because the same user may have multiple simultaneous sessions or non-SSH services. TTY is best effort: the accepted-login line often appears before a PTY is allocated.

Known limitations:

- `sudo` and `su` change UID; ancestry remains useful, but user fields may show the effective account.
- `tmux` and `screen` servers may outlive or bridge sessions, making a single-session attribution ambiguous.
- Detached/background processes can continue after logout. VMLens retains their original ancestry attribution.
- Reparenting to PID 1 before observation can break the chain.
- Very short processes can occur between `/proc` polling samples. The eBPF exec path is intended to close this gap.
- Container PID namespaces and cgroups need additional namespace-aware support in a future release.
- Logout formats vary across OpenSSH/distributions; unmatched records can leave a session shown as active.
