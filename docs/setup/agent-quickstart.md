# VMLens agent quickstart

Install the VMLens agent on a VM from a prebuilt release. Nothing is compiled on
the VM: no Go, no clang, no kernel headers.

Read this once end to end before starting. It takes about five minutes per VM.

## 1. Check the VM can run the agent

Run these four commands on the VM. All four must pass.

```bash
uname -m                          # x86_64 or aarch64
test -r /sys/kernel/btf/vmlinux && echo "BTF ok"
df -h /                           # needs at least 50 MB free
ip route show default             # note the interface name, usually ens3
```

`uname -m` decides which asset you download in step 3: `x86_64` takes `amd64`,
`aarch64` takes `arm64`.

**BTF is not optional.** Without `/sys/kernel/btf/vmlinux` the agent cannot load
its eBPF program. Ubuntu 20.04 and newer normally have it.

**Free disk is the failure we hit most.** The agent needs about 8 MB installed
plus roughly 10 MB of temporary download space. A full root filesystem produces
a confusing error, so check it up front. See troubleshooting below.

## 2. Point the agent at the control plane

The agent posts telemetry to one HTTP endpoint. Pick whichever applies.

**The control plane is reachable from the VM** — the normal case:

```text
BACKEND_URL=http://<control-plane-host>:8080
```

**The control plane runs on a laptop behind NAT** — the lab case. Open a reverse
SSH tunnel from the laptop first, then the VM uses its own localhost:

```bash
# on the laptop, not the VM
bash scripts/vmlens-tunnel.sh start <vm-alias>
```

```text
BACKEND_URL=http://127.0.0.1:18080
```

Confirm it before installing anything. On the VM:

```bash
curl -sf $BACKEND_URL/health
```

Expected: `{"database":"ok","status":"ok",...}`. If this fails, fix it first —
the agent cannot register without it.

## 3. Install

Run on the VM. Replace `amd64` with `arm64` on an ARM machine, and set
`BACKEND_URL` to what you confirmed in step 2.

```bash
curl -fsSL -o /tmp/vmlens-install-agent.sh \
  https://github.com/sonix03/vmlens-ebpf/releases/latest/download/install-agent.sh

sudo env \
  INSTALL_MODE=prebuilt \
  REQUIRE_CHECKSUM=true \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/flow_tracker-linux-amd64.bpf.o \
  bash /tmp/vmlens-install-agent.sh
```

Keep `REQUIRE_CHECKSUM=true`. It verifies both downloads against the release's
published SHA256SUMS and refuses to install anything that does not match. This
is not theoretical: it caught a truncated download during our own rollout.

A successful run ends with:

```text
Agent installer: checksum verified for vmlens-agent-linux-amd64
Agent installer: checksum verified for flow_tracker-linux-amd64.bpf.o
TC/eBPF tracker installed; logs: journalctl -u vmlens-agent -f
```

The installer writes `/etc/vmlens/agent.env`, installs a systemd unit, and
starts it. It also detects the interface automatically if `ens3` does not exist,
and excludes your own SSH client IP from captured traffic so the management
session does not show up as VM traffic.

## 4. Check it worked

On the VM:

```bash
systemctl is-active vmlens-agent
sudo journalctl -u vmlens-agent -n 20 --no-pager
```

Two lines matter:

```text
registered agent=... vm=... hostname=... mock=false
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o mode=tc interface=ens3
```

`registered` means the control plane was reached. `eBPF collector loaded` means
the kernel accepted the program. If you see the first but not the second, the
agent is running blind.

From the control plane:

```bash
curl -s http://localhost:8080/api/agents
```

Each agent should be `"status":"online"` with an `agent_version` matching the
release you installed. **A version of `dev` means that binary did not come from
a release** — someone built it by hand, and it is not the code you think it is.

To confirm traffic is really being captured, generate some between two VMs:

```bash
# on VM A
python3 -m http.server 8081 --bind 0.0.0.0

# on VM B
curl -s http://<VM-A-IP>:8081/ >/dev/null
```

Within about ten seconds the pair appears in the dashboard topology, and
`/api/internal/activity` shows the flow.

## 5. Busy VMs

The default `FLOW_INTERVAL=1s` is meant for a quiet lab. On a VM carrying real
traffic — a Redis client, a Kubernetes node, anything with steady connection
churn — add these to the install command:

```bash
FLOW_INTERVAL=5s
IGNORE_PORTS=18080,18081,18082,6379   # add ports whose traffic you already understand
```

`5s` cuts the number of posts and database rows roughly fivefold. The lost time
resolution does not matter for mapping dependencies.

Install on **one** busy VM first and watch it for an hour before doing more.
Two numbers tell you whether it is coping:

```bash
sudo bpftool map dump name capture_stats   # key 10 is the dropped-event counter
ps -o rss=,pcpu= -C vmlens-agent           # resident memory and CPU
```

Key 10 must stay at `0`, and RSS must stay flat. If either climbs, stop the
agent and report it — the capture is not keeping up with the traffic.

### What a dropped event means

If the kernel produces events faster than the agent can drain them, the extra
events are discarded. Nothing on the VM is affected: no traffic is touched, no
service is slowed, nothing is corrupted, and capture returns to normal by itself
once the load drops. What suffers is only VMLens's own record, and it always
errs in one direction — it undercounts, never overcounts.

The practical consequence:

> What the dashboard shows is real. What it does not show may simply not have
> been seen yet. Never read an empty dashboard as "there is no traffic".

## 6. Troubleshooting

Four failures account for nearly everything, and all four have a clear symptom.

### `checksum mismatch` or `curl: (23) Failure writing output`

**The disk is full.** This is what a truncated download looks like. Check:

```bash
df -h /
```

If root is at 100%, reclaim cache and rotated logs only:

```bash
sudo apt-get clean
sudo journalctl --vacuum-time=2d
sudo rm -f /tmp/vmlens-agent.prebuilt /tmp/flow_tracker.bpf.o.prebuilt
```

Then run the install again. If that does not free enough, the VM needs a bigger
disk — do not delete anything else to make room for a monitoring agent.

### `Error: remote port forwarding failed for listen port 18080`

Only applies to the tunnel setup. A previous tunnel died without releasing the
port. On the VM, find what holds it and stop only that process:

```bash
sudo ss -tlnp | grep 18080
sudo kill <pid>
```

The process is an `sshd` session with no terminal. Do not kill sessions showing
a `pts/N` — those are people's shells.

### `registered` appears but `eBPF collector loaded` does not

The eBPF program did not load. Almost always missing BTF:

```bash
test -r /sys/kernel/btf/vmlinux || echo "no BTF: agent cannot capture"
```

A kernel without BTF needs `MOCK_MODE=true` for a connectivity-only install, or
a newer kernel for real capture.

### The agent is running but the dashboard shows nothing

Check the direction of the problem before digging:

```bash
curl -sf $BACKEND_URL/health          # on the VM: can it reach the control plane
curl -s http://localhost:8080/api/agents   # on the control plane: did it register
```

If the VM reaches the control plane but no agent is listed, read the agent log —
it prints the HTTP status the control plane returned.

## 7. Removing the agent

```bash
sudo systemctl disable --now vmlens-agent
sudo rm -f /etc/systemd/system/vmlens-agent.service /usr/local/bin/vmlens-agent
sudo rm -rf /etc/vmlens /usr/lib/vmlens
sudo systemctl daemon-reload
```

The agent attaches to the kernel through TCX and kprobes; stopping the service
detaches everything. Nothing persists across the uninstall.

## What the agent reads

Packet metadata only: addresses, ports, protocol, direction, byte and packet
counts, TCP connection state, RTT and retransmissions from socket state. The one
exception is the HTTP/1.x status line, from which it keeps only the status
integer. It never reads request bodies, headers, URLs, or user buffers, and
encrypted traffic yields no application data at all.
