# Clean test: local dashboard, VM agents direct to local backend

Use this when:

- local machine runs the dashboard only;
- VM `10.20.20.130` and VM `10.20.20.199` run the tracker agent;
- no reverse SSH tunnel is used;
- both VMs can reach the local backend directly at `10.20.20.125:8080`.

## 1. Local: reset to empty state

This deletes local PostgreSQL test data.

```bash
docker compose down -v
docker compose up -d --build
```

Validate clean state:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/graph
```

Expected:

```json
[]
```

for agents and VMs, and:

```json
{"nodes":[],"edges":[]}
```

for graph.

Open:

```text
http://localhost:3000
```

## 2. SSH normally to each VM

```powershell
ssh -i "$env:USERPROFILE\.ssh\id_ed25519_vmlens" -o IdentitiesOnly=yes ubuntu@10.20.20.130
```

```powershell
ssh -i "$env:USERPROFILE\.ssh\id_ed25519_vmlens" -o IdentitiesOnly=yes ubuntu@10.20.20.199
```

## 3. From each VM: validate backend reachability

Run on both `10.20.20.130` and `10.20.20.199`:

```bash
curl http://10.20.20.125:8080/health
```

If this fails, direct mode cannot work yet. Fix routing/firewall first, or use
`scripts/vmlens-tunnel.sh`.

If the local backend runs inside WSL and only `127.0.0.1:8080` works, expose it
from Windows with portproxy. Open PowerShell as Administrator:

```powershell
wsl hostname -I
```

Use the WSL IP as `connectaddress`:

```powershell
netsh interface portproxy add v4tov4 listenaddress=10.20.20.125 listenport=8080 connectaddress=<WSL_IP> connectport=8080
netsh advfirewall firewall add rule name="VMLens backend 8080" dir=in action=allow protocol=TCP localport=8080
netsh interface portproxy show all
```

Then test again:

```bash
curl http://10.20.20.125:8080/health
```

## 4. From each VM: clone and start tracker

Run on both VMs:

```bash
sudo apt-get update
sudo apt-get install -y git golang-go clang bpftool libbpf-dev
```

```bash
git clone <repo-url>
cd vmlens-ebpf
```

```bash
BACKEND_URL=http://10.20.20.125:8080 \
bash scripts/vmlens-agent.sh start
```

Check:

```bash
bash scripts/vmlens-agent.sh status
bash scripts/vmlens-agent.sh logs
```

## 5. Local: validate both VMs are online

```bash
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/graph
```

Expected:

- two online agents;
- two VM nodes: `10.20.20.130` and `10.20.20.199`;
- no edge yet until VM-to-VM traffic happens.

## 6. Generate VM-to-VM traffic

On VM `10.20.20.199`:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

On VM `10.20.20.130`:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null http://10.20.20.199:8081/; sleep 0.2; done
```

Reverse direction:

On VM `10.20.20.130`:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

On VM `10.20.20.199`:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null http://10.20.20.130:8081/; sleep 0.2; done
```

## 7. Local: validate realtime result

```bash
curl 'http://127.0.0.1:8080/api/internal/activity?limit=20'
curl http://127.0.0.1:8080/api/stats/summary
curl http://127.0.0.1:8080/api/graph
```

Expected:

- frontend shows two VM nodes;
- line animates during traffic;
- internal traffic bytes increase;
- request frequency increases briefly.

## 8. Stop

On each VM:

```bash
bash scripts/vmlens-agent.sh stop
```

Local:

```bash
docker compose down
```
