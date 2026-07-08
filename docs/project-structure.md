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
```

Local files are ignored by git:

```text
configs/local.env
configs/vms.local
```

Use `configs/local.env` for local paths and defaults:

```text
VMLENS_SSH_USER
VMLENS_SSH_KEY
VMLENS_LOCAL_BACKEND
VMLENS_REMOTE_BACKEND
VMLENS_VM_PROFILES
VMLENS_VM_<PROFILE>_ALIAS
VMLENS_VM_<PROFILE>_HOST
VMLENS_VM_<PROFILE>_SSH_USER
VMLENS_VM_<PROFILE>_SSH_KEY
VMLENS_VM_INVENTORY
VMLENS_TUNNEL_STATE_DIR
VMLENS_KEY_STATE_DIR
```

Example:

```text
VMLENS_VM_PROFILES="testing_a_1 testing_a_2"

VMLENS_VM_TESTING_A_1_ALIAS=testing-a-1
VMLENS_VM_TESTING_A_1_HOST=10.20.20.130
VMLENS_VM_TESTING_A_1_SSH_KEY=-

VMLENS_VM_TESTING_A_2_ALIAS=testing-a-2
VMLENS_VM_TESTING_A_2_HOST=10.20.20.199
VMLENS_VM_TESTING_A_2_SSH_KEY=~/.ssh/id_ed25519_vmlens_testing_a_2
```

`configs/vms.local` still exists as a legacy fallback when
`VMLENS_VM_PROFILES` is empty:

```text
testing-a-1|10.20.20.130|-|-|-|-
testing-a-2|10.20.20.199|ubuntu|~/.ssh/id_ed25519_vmlens|-|-
testing-a-3|10.20.20.188|ubuntu|agent|-|-
```
