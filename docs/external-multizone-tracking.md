# External multi-zone VM tracking

Goal:

- track internal VMs with VMLens;
- let those VMs communicate with a real external service VM;
- keep the external service VM counted as external traffic, not internal traffic.

Important rule:

Do not install `vmlens-agent` on the external service VM. If the external VM is
registered in VMLens, the backend correctly treats it as an internal/registered
VM.

## Local control-plane

Default classification is already safe:

```bash
UNREGISTERED_INTERNAL_SCOPE=external_private docker compose up -d --build
```

Check:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/stats/summary
```

## Internal VM agent

Use Traffic Control capture on `ens3`:

```bash
sudo env \
  INSTALL_MODE=prebuilt \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=1s \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  AGENT_IGNORE_IPS=10.20.20.125 \
  FLOW_DENY_CIDRS=10.20.20.125/32 \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/flow_tracker-linux-amd64.bpf.o \
  bash /tmp/vmlens-install-agent.sh
```

Check:

```bash
systemctl is-active vmlens-agent
sudo journalctl -u vmlens-agent -n 80 --no-pager
ip -brief address show ens3
```

Expected log:

```text
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o mode=tc interface=ens3
```

If the VM does not support TCX, use fallback mode:

```bash
sudo sed -i 's/^CAPTURE_MODE=.*/CAPTURE_MODE=auto/' /etc/vmlens/agent.env
sudo systemctl restart vmlens-agent
```

## External service VM

Do not install VMLens agent here.

Run a simple TCP service:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

## Internal VM client test

From a tracked internal VM:

```bash
python3 -c "import time, urllib.request
target='http://EXTERNAL_VM_IP:8081/'
for i in range(1, 31):
    r = urllib.request.urlopen(target, timeout=5)
    print(f'request={i:02d} status={r.status}')
    r.read(128)
    time.sleep(0.2)
"
```

## Local verification

```bash
curl 'http://127.0.0.1:8080/api/graph?scope=external_private&time_range=24h'
curl http://127.0.0.1:8080/api/stats/summary
```

Expected:

- the external service VM appears as an `external` node;
- flow scope is `external_private`;
- `external_flows` and `external_bytes` increase;
- `internal_flows` does not increase for the external target.

## DeepFlow coexistence check

Run on the VM where DeepFlow is installed:

```bash
systemctl status deepflow-agent --no-pager
sudo bpftool prog show | grep -Ei 'deepflow|vmlens|tc|trace' || true
sudo bpftool link show | grep -Ei 'deepflow|vmlens|tcx|tc' || true
sudo tc filter show dev ens3 ingress || true
sudo tc filter show dev ens3 egress || true
```

If DeepFlow already owns an incompatible TC hook on the same interface, run
VMLens in fallback socket mode:

```bash
sudo sed -i 's/^CAPTURE_MODE=.*/CAPTURE_MODE=kprobe/' /etc/vmlens/agent.env
sudo systemctl restart vmlens-agent
```
