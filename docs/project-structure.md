# Project Structure

VMLens uses professional service names in Docker and documentation, while the
current source folders remain stable for compatibility.

## Runtime services

```text
dashboard      browser UI on http://localhost:3000
control-plane  REST API, SSE, graph/status logic on http://localhost:8080
datastore      PostgreSQL on localhost:5432
agent          VM-side eBPF telemetry collector
```

## Source folders

```text
frontend/      source for the dashboard service
backend/       source for the control-plane service
agent/         source for the VM-side telemetry agent
scripts/       local tunnel and VM agent management scripts
configs/       local operator configuration examples
instructions/  copy-paste traffic and setup recipes
docs/          architecture and operations notes
```

The folders `frontend/` and `backend/` are intentionally kept for now because
they are referenced by existing Docker build contexts, module paths, and setup
history. Prefer these service names in user-facing docs:

```text
frontend  -> dashboard
backend   -> control-plane
postgres  -> datastore
```

## Local operator files

Copy examples before editing local machine-specific values:

```bash
cp configs/local.env.example configs/local.env
cp configs/vms.example configs/vms.local
```

Local files are ignored by git:

```text
configs/local.env
configs/vms.local
```

Use `configs/local.env` for local paths and defaults:

```text
VMLENS_SSH_USER
VMLINUX_SSH_KEY
VMLINUX_LOCAL_BACKEND
VMLINUX_REMOTE_BACKEND
VMLINUX_VM_INVENTORY
VMLINUX_TUNNEL_STATE_DIR
VMLINUX_KEY_STATE_DIR
```

Use `configs/vms.local` for VM inventory:

```text
alias|host|ssh_user|ssh_key|remote_backend|local_backend
```

Examples:

```text
testing-a-1|10.20.20.130|-|-|-|-
testing-a-2|10.20.20.199|ubuntu|~/.ssh/id_ed25519_vmlens|-|-
testing-a-3|10.20.20.188|ubuntu|agent|-|-
```
