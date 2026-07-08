# VMLens OpenStack Configuration

Use this when creating a new OpenStack VM. Paste the content into:

```text
Customization Script
```

OpenStack calls this feature `Customization Script`. It is the same idea as
`User Data` or `cloud-init` in other clouds.

## OpenStack Customization Script

```yaml
#cloud-config
package_update: true
package_upgrade: false

write_files:
  - path: /usr/local/sbin/vmlens-bootstrap.sh
    permissions: "0755"
    content: |
      #!/usr/bin/env bash
      set -euxo pipefail

      REPO_URL="https://github.com/sonix03/vmlens-ebpf.git"
      REPO_DIR="/opt/vmlens-ebpf"

      BACKEND_URL="http://127.0.0.1:18080"
      MOCK_MODE="false"
      FLOW_INTERVAL="1s"

      # Optional. Add local/tunnel/control-plane IPs here if they should not be tracked.
      # Example: AGENT_IGNORE_IPS="10.20.20.125"
      AGENT_IGNORE_IPS=""

      export DEBIAN_FRONTEND=noninteractive

      apt-get update
      apt-get install -y ca-certificates git golang-go clang libbpf-dev linux-tools-common

      apt-get install -y "linux-tools-$(uname -r)" || true
      apt-get install -y bpftool || true

      if ! command -v bpftool >/dev/null 2>&1; then
        BPFTOOL_PATH="$(find /usr/lib/linux-tools* -name bpftool -type f 2>/dev/null | head -n1 || true)"
        if [ -n "$BPFTOOL_PATH" ]; then
          ln -sf "$BPFTOOL_PATH" /usr/local/bin/bpftool
        fi
      fi

      command -v bpftool >/dev/null
      test -r /sys/kernel/btf/vmlinux

      rm -rf "$REPO_DIR"
      git clone "$REPO_URL" "$REPO_DIR"

      cd "$REPO_DIR"
      chmod +x scripts/*.sh

      env \
        BACKEND_URL="$BACKEND_URL" \
        MOCK_MODE="$MOCK_MODE" \
        FLOW_INTERVAL="$FLOW_INTERVAL" \
        AGENT_IGNORE_IPS="$AGENT_IGNORE_IPS" \
        bash scripts/vmlens-agent.sh start

runcmd:
  - [ bash, /usr/local/sbin/vmlens-bootstrap.sh ]
```

## Local dashboard and tunnel

Run on local:

```bash
cd /mnt/c/Documents/Ionext/vmlens-ebpf
docker compose up -d --build
```

Create local operator config:

```bash
cp configs/local.env.example configs/local.env
```

Edit:

```text
configs/local.env
```

Put the VM list and SSH key mapping in `configs/local.env`.

List configured VMs:

```bash
bash scripts/vmlens-tunnel.sh list
```

Start all tunnels:

```bash
bash scripts/vmlens-tunnel.sh start-all
```

Or start one tunnel per VM by alias or IP:

```bash
bash scripts/vmlens-tunnel.sh start testing-a-1
bash scripts/vmlens-tunnel.sh start <VM_IP_OR_HOST>
```

Example:

```bash
bash scripts/vmlens-tunnel.sh start 10.20.20.130
bash scripts/vmlens-tunnel.sh start 10.20.20.199
```

Aliases from `VMLENS_VM_PROFILES` also work:

```bash
bash scripts/vmlens-tunnel.sh start testing-a-1
bash scripts/vmlens-tunnel.sh start testing-a-2
```

Open:

```text
http://localhost:3000
```

## SSH key model

One shared SSH key for all VMs is normal if the same public key is installed on
each VM.

Example:

```text
VMLENS_SSH_USER=ubuntu
VMLENS_SSH_KEY=~/.ssh/id_ed25519_vmlens

VMLENS_VM_PROFILES="testing_a_1 testing_a_2 testing_a_3"

VMLENS_VM_TESTING_A_1_ALIAS=testing-a-1
VMLENS_VM_TESTING_A_1_HOST=10.20.20.130
VMLENS_VM_TESTING_A_1_SSH_USER=-
VMLENS_VM_TESTING_A_1_SSH_KEY=-

VMLENS_VM_TESTING_A_2_ALIAS=testing-a-2
VMLENS_VM_TESTING_A_2_HOST=10.20.20.199
VMLENS_VM_TESTING_A_2_SSH_USER=-
VMLENS_VM_TESTING_A_2_SSH_KEY=-
```

The `-` values mean: use defaults from `configs/local.env`.

Per-VM keys are also supported:

```text
VMLENS_VM_TESTING_A_1_SSH_KEY=~/.ssh/id_ed25519_vmlens_a1
VMLENS_VM_TESTING_A_2_SSH_KEY=~/.ssh/id_ed25519_vmlens_a2
```

If SSH already works through `~/.ssh/config` or `ssh-agent`, set the key to
`agent` or `none`:

```text
VMLENS_VM_TESTING_A_3_SSH_KEY=agent
```

Password-based SSH may prompt interactively, but key-based access is recommended
for stable reverse tunnels.

## What happens if the tunnel is not ready yet

The customization script runs once when the VM is created.

It does this once:

```text
install packages
clone repository
build agent
install systemd service
start vmlens-agent
```

After that, `vmlens-agent` runs as a systemd service.

If `BACKEND_URL=http://127.0.0.1:18080` is not reachable yet because the local
reverse SSH tunnel is not running, the agent does not rerun the whole bootstrap
script. Only the agent process retries registration.

Retry delay:

```text
1s -> 2s -> 4s -> 8s -> 16s -> 30s
```

Then it keeps retrying about every 30 seconds until the backend is reachable.

When the tunnel becomes available, the agent registers automatically and the VM
node appears in the frontend.

## Check from the VM

Check service status:

```bash
sudo systemctl status vmlens-agent --no-pager
```

Follow logs:

```bash
sudo journalctl -u vmlens-agent -f
```

Check backend tunnel from inside the VM:

```bash
curl http://127.0.0.1:18080/health
```

Expected after tunnel is ready:

```json
{"database":"ok","status":"ok"}
```

## Restart agent

```bash
sudo systemctl restart vmlens-agent
```

## Stop agent

```bash
sudo systemctl stop vmlens-agent
```

## Disable auto-start on reboot

```bash
sudo systemctl disable vmlens-agent
```

## Uninstall agent

```bash
cd /opt/vmlens-ebpf
sudo bash scripts/uninstall-agent.sh
```

## Notes

`MOCK_MODE=false` means real eBPF capture.

`BACKEND_URL=http://127.0.0.1:18080` works inside the VM only after the local
machine creates a reverse SSH tunnel.

Traffic is tracked at VM network level. For Kubernetes, Redis, PostgreSQL, and
other services, the UI shows VM-to-VM communication and known ports such as:

```text
6379  redis
5432  postgresql
6443  kubernetes-api
30000-32767 kubernetes-nodeport
```
