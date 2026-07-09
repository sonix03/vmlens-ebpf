VMLens Simple Setup
===================

Goal:
- Local laptop runs dashboard, backend, and database with Docker.
- Cloud VMs run the VMLens agent.
- VM agent uses release v2.7 prebuilt binary.
- VM does not need git clone or compile.

Architecture:

Cloud VM agent -> reverse SSH tunnel -> local backend -> dashboard


1. Local: start dashboard
=========================

Run on local:

docker compose up -d --build

Check:

curl http://127.0.0.1:8080/health
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms

Open:

http://localhost:3000


2. Local: start tunnel to each VM
=================================

Run on local:

bash scripts/vmlens-tunnel.sh start 10.20.20.130 ~/.vmlens/keys/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh start 10.20.20.199 ~/.vmlens/keys/id_ed25519_vmlens

Check:

bash scripts/vmlens-tunnel.sh status 10.20.20.130 ~/.vmlens/keys/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh status 10.20.20.199 ~/.vmlens/keys/id_ed25519_vmlens

Expected:

running


3. VM: check backend tunnel
===========================

Run on each VM:

curl http://127.0.0.1:18080/health

Expected:

{"database":"ok","status":"ok",...}


4. VM: install VMLens agent from release
========================================

Run on each VM:

curl -fsSL -o /tmp/vmlens-install-agent.sh \
  https://github.com/sonix03/vmlens-ebpf/releases/download/v2.7/install-agent.sh

chmod +x /tmp/vmlens-install-agent.sh

sudo env \
  INSTALL_MODE=prebuilt \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  FLOW_INTERVAL=1s \
  AGENT_BINARY_URL=https://github.com/sonix03/vmlens-ebpf/releases/download/v2.7/vmlens-agent-linux-amd64 \
  BPF_OBJECT_URL=https://github.com/sonix03/vmlens-ebpf/releases/download/v2.7/flow_tracker-linux-amd64.bpf.o \
  bash /tmp/vmlens-install-agent.sh

Check:

systemctl is-active vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 50 --no-pager

Expected:

active
registered agent=...
eBPF collector loaded object=/usr/lib/vmlens/flow_tracker.bpf.o


5. Local: verify VM nodes
=========================

Run on local:

curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/stats/summary

Expected:

agents online
VMs online


6. Test VM-to-VM traffic
========================

Server VM, example 10.20.20.199:

python3 -m http.server 8081 --bind 0.0.0.0

Client VM, example 10.20.20.130:

python3 -c "import time, urllib.request
for i in range(1, 21):
    r = urllib.request.urlopen('http://10.20.20.199:8081/', timeout=5)
    print(f'request={i:02d} status={r.status}')
    r.read(128)
    time.sleep(0.2)
"

Expected:

request=01 status=200
...
request=20 status=200


7. Local: verify tracking
=========================

Run on local:

curl http://127.0.0.1:8080/api/stats/summary
curl 'http://127.0.0.1:8080/api/internal/activity?limit=10'
curl http://127.0.0.1:8080/api/graph

Expected:

internal_flows increases
network_requests_total increases
internal activity shows VM-to-VM communication

Example:

testing-a-2 (10.20.20.130) -> testing-a-1 (10.20.20.199):8081 tcp


8. Stop
=======

Stop agent on each VM:

sudo systemctl stop vmlens-agent

Stop tunnels on local:

bash scripts/vmlens-tunnel.sh stop 10.20.20.130 ~/.vmlens/keys/id_ed25519_vmlens
bash scripts/vmlens-tunnel.sh stop 10.20.20.199 ~/.vmlens/keys/id_ed25519_vmlens

Stop local dashboard:

docker compose down

Reset local database:

docker compose down -v


9. Clean reinstall
==================

If you want to retry from zero, run on each VM:

sudo systemctl stop vmlens-agent 2>/dev/null || true
sudo systemctl disable vmlens-agent 2>/dev/null || true
sudo rm -f /etc/systemd/system/vmlens-agent.service
sudo rm -f /usr/local/bin/vmlens-agent
sudo rm -f /etc/vmlens/agent.env
sudo rm -f /usr/lib/vmlens/flow_tracker.bpf.o
sudo systemctl daemon-reload

Then repeat from step 1.


10. Release assets
==================

Release:

https://github.com/sonix03/vmlens-ebpf/releases/tag/v2.7

Assets used:

vmlens-agent-linux-amd64
flow_tracker-linux-amd64.bpf.o
install-agent.sh
SHA256SUMS
