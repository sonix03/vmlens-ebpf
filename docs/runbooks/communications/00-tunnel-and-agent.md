# Tunnel dan agent

```bash
# Local
docker compose up -d
bash scripts/vmlens-tunnel.sh start 10.20.20.130
bash scripts/vmlens-tunnel.sh start 10.20.20.199
```

```bash
# VM
git clone <repo-url>
cd vmlens-ebpf
bash scripts/vmlens-agent.sh start
```

```bash
# VM
curl http://127.0.0.1:18080/health
bash scripts/vmlens-agent.sh status
bash scripts/vmlens-agent.sh logs
```

```bash
# Stop
bash scripts/vmlens-agent.sh stop
bash scripts/vmlens-tunnel.sh stop 10.20.20.130
bash scripts/vmlens-tunnel.sh stop 10.20.20.199
```
