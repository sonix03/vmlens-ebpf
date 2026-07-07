# Cloud VM setup

## Local

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/health
```

```bash
bash scripts/vmlens-tunnel.sh start <VM_A_PUBLIC_IP>
bash scripts/vmlens-tunnel.sh start <VM_B_PUBLIC_IP>
```

```text
http://localhost:3000
```

## VM A and VM B

```bash
sudo apt-get update
sudo apt-get install -y git golang-go clang bpftool libbpf-dev
```

```bash
git clone <repo-url>
cd vmlens-ebpf
```

```bash
BACKEND_URL=http://127.0.0.1:18080 \
AGENT_PUBLIC_IP=<THIS_VM_PUBLIC_IP> \
bash scripts/vmlens-agent.sh start
```

```bash
bash scripts/vmlens-agent.sh status
bash scripts/vmlens-agent.sh logs
```

## Test VM A to VM B

```bash
# VM B
python3 -m http.server 8081 --bind 0.0.0.0
```

```bash
# VM A
for i in $(seq 1 20); do curl -s -o /dev/null http://<VM_B_PUBLIC_IP>:8081/; sleep 0.2; done
```

## Check local

```bash
curl http://127.0.0.1:8080/api/agents
curl 'http://127.0.0.1:8080/api/internal/activity?limit=20'
curl http://127.0.0.1:8080/api/stats/summary
```

## Stop

```bash
# VM
bash scripts/vmlens-agent.sh stop
```

```bash
# Local
bash scripts/vmlens-tunnel.sh stop <VM_A_PUBLIC_IP>
bash scripts/vmlens-tunnel.sh stop <VM_B_PUBLIC_IP>
docker compose down
```
