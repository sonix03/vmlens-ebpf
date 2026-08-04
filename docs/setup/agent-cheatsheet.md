# VMLens cheat sheet

Copy-paste order. Details: `agent-quickstart.md`.

## 1. Local — start the stack

```bash
docker compose up -d
curl -s http://localhost:8080/health
```

## 2. Local — open the tunnel

```bash
bash scripts/vmlens-tunnel.sh start <vm-alias>
bash scripts/vmlens-tunnel.sh status <vm-alias>
```

## 3. VM — install

```bash
curl -sf http://127.0.0.1:18080/health

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

## 4. VM — check

```bash
systemctl is-active vmlens-agent
sudo journalctl -u vmlens-agent -n 20 --no-pager
sudo bpftool map dump name capture_stats
```

Want: `active`, `registered agent=...`, `eBPF collector loaded ... mode=tc`,
`capture_stats` key 10 = 0.

## 5. Local — look

```bash
curl -s http://localhost:8080/api/agents
```

http://localhost:3000

## Uninstall

```bash
sudo systemctl disable --now vmlens-agent
sudo rm -f /etc/systemd/system/vmlens-agent.service /usr/local/bin/vmlens-agent
sudo rm -rf /etc/vmlens /usr/lib/vmlens
sudo systemctl daemon-reload
```
