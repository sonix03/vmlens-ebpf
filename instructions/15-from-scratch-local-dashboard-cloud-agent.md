# From scratch: local dashboard tracks cloud VM

This setup uses:

- local machine/VM: dashboard only, via Docker Compose;
- cloud VM: tracked VM, via `vmlens-agent`;
- no agent on the local machine/VM.

Tracking runs directly inside the cloud VM. The local machine only receives and
displays telemetry.

With one cloud VM, you can track the VM node and external traffic. To see an
internal VM-to-VM arrow, install the agent on two cloud VMs.

## 1. Local machine/VM

```bash
git clone <repo-url>
cd vmlens-ebpf
```

```bash
docker compose up -d --build
curl http://127.0.0.1:8080/health
```

```text
http://localhost:3000
```

Start one reverse tunnel to the cloud VM:

```bash
bash scripts/vmlens-tunnel.sh start <CLOUD_VM_PUBLIC_IP>
```

The cloud VM will reach the local backend through:

```text
http://127.0.0.1:18080
```

This tunnel only sends cloud VM telemetry back to local. It does not track the
local machine.

Or run the full local preflight:

```bash
bash scripts/check-cloud-vm.sh <CLOUD_VM_PUBLIC_IP>
```

This checks:

- local backend health;
- SSH access to the cloud VM;
- reverse tunnel startup;
- backend health from inside the cloud VM.

## 2. Cloud VM

SSH normally:

```bash
ssh ubuntu@<CLOUD_VM_PUBLIC_IP>
```

Clone and install the tracker inside the cloud VM:

```bash
sudo apt-get update
sudo apt-get install -y git golang-go clang bpftool libbpf-dev
```

```bash
git clone <repo-url>
cd vmlens-ebpf
```

Start the agent:

```bash
BACKEND_URL=http://127.0.0.1:18080 \
AGENT_PUBLIC_IP=<CLOUD_VM_PUBLIC_IP> \
bash scripts/vmlens-agent.sh start
```

Check the agent:

```bash
bash scripts/vmlens-agent.sh status
bash scripts/vmlens-agent.sh logs
```

## 3. Validate from local

```bash
curl http://127.0.0.1:8080/api/agents
curl http://127.0.0.1:8080/api/vms
curl http://127.0.0.1:8080/api/stats/summary
```

Expected:

- the cloud VM agent is `online`;
- the cloud VM appears in the dashboard;
- local machine/VM does not appear as a tracked VM.

## 4. Generate traffic on the cloud VM

Run this on the cloud VM:

```bash
curl --fail --show-error -o /dev/null \
  -w 'status=%{http_code} downloaded=%{size_download}\n' \
  'https://speed.cloudflare.com/__down?bytes=10485760'
```

Check from local:

```bash
curl http://127.0.0.1:8080/api/stats/summary
curl 'http://127.0.0.1:8080/api/internal/activity?limit=20'
```

Expected:

- `External traffic` increases;
- `Request frequency` increases briefly;
- no internal VM-to-VM arrow appears unless another tracked VM is involved.

## 5. Optional: internal VM-to-VM arrow

Repeat steps 1-3 for a second cloud VM:

```bash
bash scripts/vmlens-tunnel.sh start <CLOUD_VM_B_PUBLIC_IP>
```

Install the agent on VM B with:

```bash
BACKEND_URL=http://127.0.0.1:18080 \
AGENT_PUBLIC_IP=<CLOUD_VM_B_PUBLIC_IP> \
bash scripts/vmlens-agent.sh start
```

On VM B:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

On VM A:

```bash
for i in $(seq 1 20); do curl -s -o /dev/null http://<CLOUD_VM_B_PUBLIC_IP>:8081/; sleep 0.2; done
```

Expected:

- VM A and VM B both appear as nodes;
- the VM A ↔ VM B line animates during traffic;
- bytes and request frequency increase in real time.

## 6. Stop

On the cloud VM:

```bash
bash scripts/vmlens-agent.sh stop
```

On local:

```bash
bash scripts/vmlens-tunnel.sh stop <CLOUD_VM_PUBLIC_IP>
docker compose down
```
