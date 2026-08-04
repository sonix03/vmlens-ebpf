# VMLens cheat sheet

Copy-paste order. Reasoning and troubleshooting: `agent-quickstart.md`.

Replace `<VM-IP>` and `<KEY>` throughout. `<KEY>` is the private key that reaches
the VM, e.g. `~/.vmlens/keys/id_ed25519_vmlens`. Pass it every time: the alias
form only works with `configs/vms.local`, which is not in the repo.

Replace `amd64` with `arm64` on an ARM VM (`uname -m` says `aarch64`).

## 1. Local: start the stack

```bash
docker compose up -d --build

curl http://127.0.0.1:8080/health
```

Open http://localhost:3000

## 2. Local: tunnel to each VM

```bash
bash scripts/vmlens-tunnel.sh start <VM-IP> <KEY>

bash scripts/vmlens-tunnel.sh status <VM-IP> <KEY>
```

Expected: `running`

## 3. VM: check the tunnel reaches the backend

```bash
curl http://127.0.0.1:18080/health
```

Expected: `{"database":"ok","status":"ok",...}`

Do not install until this works.

## 4. VM: install the agent

```bash
curl -fsSL -o /tmp/vmlens-install-agent.sh \
  https://github.com/sonix03/vmlens-ebpf/releases/latest/download/install-agent.sh

sudo env \
  INSTALL_MODE=prebuilt \
  REQUIRE_CHECKSUM=true \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=5s \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/latest/download/flow_tracker-linux-amd64.bpf.o \
  bash /tmp/vmlens-install-agent.sh
```

Expected:

```text
Agent installer: checksum verified for vmlens-agent-linux-amd64
Agent installer: checksum verified for flow_tracker-linux-amd64.bpf.o
TC/eBPF tracker installed
```

## 5. VM: check the agent

```bash
systemctl is-active vmlens-agent

sudo journalctl -u vmlens-agent -n 50 --no-pager

sudo bpftool map dump name capture_stats
```

Expected:

```text
active
registered agent=...
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o mode=tc interface=ens3
capture_stats key 10 = 0
```

## 6. Local: verify the VM shows up

```bash
curl http://127.0.0.1:8080/api/agents

curl http://127.0.0.1:8080/api/vms

curl http://127.0.0.1:8080/api/stats/summary
```

Expected: agents `online`, `agent_version` matching the installed release.
`dev` means the binary was not built from a release.

## 7. Test VM-to-VM traffic

Server VM:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

Client VM:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null -w "%{http_code}\n" http://<SERVER-VM-IP>:8081/; sleep 0.2; done
```

Local:

```bash
curl 'http://127.0.0.1:8080/api/internal/activity?limit=10'

curl http://127.0.0.1:8080/api/graph
```

Expected: the VM pair appears in the topology within about ten seconds.

## 8. Stop

VM:

```bash
sudo systemctl stop vmlens-agent
```

Local:

```bash
bash scripts/vmlens-tunnel.sh stop <VM-IP> <KEY>

docker compose down
```

Reset the database as well:

```bash
docker compose down -v
```

## 9. Uninstall from the VM

```bash
sudo systemctl disable --now vmlens-agent
sudo rm -f /etc/systemd/system/vmlens-agent.service /usr/local/bin/vmlens-agent
sudo rm -rf /etc/vmlens /usr/lib/vmlens
sudo systemctl daemon-reload
```

## Release assets

https://github.com/sonix03/vmlens-ebpf/releases/latest

```text
vmlens-agent-linux-amd64
vmlens-agent-linux-arm64
flow_tracker-linux-amd64.bpf.o
flow_tracker-linux-arm64.bpf.o
install-agent.sh
SHA256SUMS
```
