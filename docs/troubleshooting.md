# Troubleshooting

## Agent does not start

Run `sudo journalctl -u vmlens -n 100 --no-pager`. Validate YAML with `sudo /usr/local/bin/vmlens run --config /etc/vmlens/vmlens.yaml`. Privacy options that imply payload, keystroke, terminal capture, or remote upload are rejected intentionally.

## Permission denied

Run the service as root. `/proc/<pid>/io`, other users' process metadata, auth logs, and eBPF attachment are privilege-controlled. Keep JSONL restricted and use a read-only group if operators need CLI access.

## Missing kernel headers, BTF, or eBPF support

Check `/sys/kernel/btf/vmlinux`, install `linux-headers-$(uname -r)`, `bpftool`, `clang`, and `libbpf-dev`, then run `make bpf`. The userspace `/proc` fallback does not require BPF objects.

## SSH sessions are absent

Check `sudo tail -f /var/log/auth.log`. If the file does not exist, ensure `ssh.parse_journald: true`, then verify `sudo journalctl -u ssh.service -u sshd.service`. Distribution-specific message formats may require another parser fixture.

## Prometheus cannot scrape

Test on the host with `curl http://127.0.0.1:9435/metrics`. A container's loopback is not the host loopback. Use host networking, or bind a controlled host interface and add firewall policy. Do not expose the endpoint publicly.

## Metrics appear merged

Process name is intentionally the main label. Multiple PIDs with the same name update the same gauge, keeping cardinality bounded. Use JSONL and `vmlens ssh inspect` for per-PID/session detail.

## Network byte values are zero

This is expected for the v0.1 socket fallback. It observes connection ownership and endpoints but not byte-accurate accounting. No packet payload is collected.
