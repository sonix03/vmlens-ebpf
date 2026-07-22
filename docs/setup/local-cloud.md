# VMLens Setup

Simple setup:

- local laptop/PC: runs the control-plane API, datastore, and dashboard UI;
- cloud VM: runs the tracker agent and captures VM traffic;
- reverse SSH tunnel: sends cloud VM telemetry back to local backend.

## 1. Local: start dashboard

Run on local:

```bash
cd /mnt/c/Documents/Ionext/vmlens-ebpf
docker compose up -d --build
```

Optional but recommended local configuration:

```bash
cp configs/local.env.example configs/local.env
```

Edit `configs/local.env` for VM list, SSH user, and per-VM SSH key path.

Check:

```bash
curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/graph
```

Open UI:

```text
http://localhost:3000
```

## 2. Local: start tunnel to cloud VM

If `configs/local.env` is configured, run:

```bash
bash scripts/vmlens-tunnel.sh list
bash scripts/vmlens-tunnel.sh start-all
```

Or run one command per VM:

```bash
bash scripts/vmlens-tunnel.sh start testing-a-1
bash scripts/vmlens-tunnel.sh start testing-a-2
bash scripts/vmlens-tunnel.sh start testing-a-3
```

Direct host/IP still works:

```bash
bash scripts/vmlens-tunnel.sh start 10.20.20.130
```

Use an explicit SSH key:

```bash
bash scripts/vmlens-tunnel.sh start 10.20.20.130 ~/.vmlens/keys/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh stop 10.20.20.130 ~/.vmlens/keys/id_ed25519_vmlens
```

Remove stale SSH host key after a VM IP is reused:

```bash
bash scripts/vmlens-tunnel.sh forget-host 10.20.20.130
```

Show the resolved key for one VM:

```bash
bash scripts/vmlens-tunnel.sh show testing-a-1
```

The VM agent will use:

```text
http://127.0.0.1:18080
```

That port exists inside the VM because of the reverse tunnel.

## 3. Cloud VM: clone repo

Run on each cloud VM:

```bash
sudo systemctl stop vmlens-agent 2>/dev/null || true
rm -rf ~/vmlens-ebpf
git clone https://github.com/sonix03/vmlens-ebpf.git
cd ~/vmlens-ebpf
chmod +x scripts/*.sh
```

## 4. Cloud VM: install dependencies

Run on each cloud VM:

```bash
sudo apt-get update
sudo apt-get install -y git golang-go clang libbpf-dev linux-tools-common linux-tools-$(uname -r)
```

If `linux-tools-$(uname -r)` is not available:

```bash
sudo apt-get install -y git golang-go clang libbpf-dev linux-tools-common
sudo find /usr/lib/linux-tools* -name bpftool -type f
```

If `bpftool` is found, create a symlink. Example:

```bash
sudo ln -sf /usr/lib/linux-tools/$(uname -r)/bpftool /usr/local/bin/bpftool
```

Check:

```bash
bpftool version
```

## 5. Cloud VM: check backend tunnel

Run on each cloud VM:

```bash
curl http://127.0.0.1:18080/health
```

Expected:

```json
{"database":"ok","status":"ok",...}
```

If this fails, start the tunnel from local again:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP>
```

## 6. Cloud VM: start tracker agent

Run on each cloud VM:

```bash
cd ~/vmlens-ebpf

sudo env BACKEND_URL=http://127.0.0.1:18080 \
MOCK_MODE=false \
bash scripts/vmlens-agent.sh start
```

Check:

```bash
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 50 --no-pager
```

## 7. Local: check tracked VMs

Run on local:

```bash
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/graph
```

Expected:

- agents are `online` after registration or heartbeat;
- VM nodes become `online` after registration, heartbeat, or observed traffic;
- VM nodes appear in the frontend.

## 8. Test VM-to-VM traffic

On VM `10.20.20.199`:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

On VM `10.20.20.130`:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null http://10.20.20.199:8081/; sleep 0.2; done
```

Check local:

```bash
curl 'http://127.0.0.1:8080/api/internal/activity?limit=20'
curl http://127.0.0.1:8080/api/stats/summary
```

Open UI:

```text
http://localhost:3000
```

Expected:

- VM nodes stay visible;
- line animates while traffic is active, and stays highlighted for a short
  active window after the latest observed request;
- internal traffic and request frequency increase.

## 9. Stop agent

Run on each cloud VM:

```bash
cd ~/vmlens-ebpf
sudo bash scripts/vmlens-agent.sh stop
```

Or:

```bash
sudo systemctl stop vmlens-agent
```

Check:

```bash
systemctl is-active vmlens-agent
```

Expected:

```text
inactive
```

Disable auto-start on reboot:

```bash
sudo systemctl disable vmlens-agent
```

Uninstall agent:

```bash
cd ~/vmlens-ebpf
sudo bash scripts/uninstall-agent.sh
```

## 10. Stop tunnel

Run on local:

```bash
bash scripts/vmlens-tunnel.sh stop-all
```

## 11. Stop local dashboard

Run on local:

```bash
docker compose down
```

Delete local database and start from zero:

```bash
docker compose down -v
docker compose up -d --build
```

## Notes

`MOCK_MODE=false` means real eBPF capture.

`MOCK_MODE=true` means fake/demo traffic; do not use it for real tracking.

`BACKEND_URL=http://127.0.0.1:18080` works inside the cloud VM because the local
machine created a reverse SSH tunnel.

## VM status lifecycle

Status is based on `last_seen`. `last_seen` is refreshed by agent registration,
heartbeat, and observed traffic.

```text
online  = last_seen is within 1 minute
stale   = last_seen is older than 1 minute but still within 5 minutes
offline = last_seen is older than 5 minutes
```

Agent heartbeat interval:

```text
20 seconds
```

Backend status sweep interval:

```text
10 seconds
```

This means:

- after agent start/register, the VM should become `online` quickly;
- after agent or tunnel stops, it usually becomes `stale` after about 1 minute;
- it usually becomes `offline` after about 5 minutes;
- if the tunnel is restored and heartbeat/register reaches the backend again,
  it becomes `online` again.
