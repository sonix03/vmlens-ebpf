# Project Structure

VMLens keeps the active product stack at the repository root and moves setup
notes, runbooks and deployment files into purpose-specific folders.

The root uses `go.work` to group the active Go modules. From the repository
root, use `make test`; run raw `go test ./...` inside `agent/` or `backend/`.

## Active root folders

```text
agent/          VM-side telemetry agent
backend/        control-plane API, graph, stats, database migrations
frontend/       dashboard UI
scripts/        active operator scripts for tunnels, agent install, release, tests
configs/        local operator config examples
deploy/         runtime/deployment assets
docs/           architecture, setup guides and operations notes
```

## Deploy layout

```text
deploy/
  openstack/     OpenStack cloud-init / Customization Script files
```

## Documentation layout

```text
docs/
  setup/         local/cloud setup guides
  runbooks/      repeatable command flows
    communications/  copy-paste VM communication test recipes
```

## Local operator config

Copy examples before editing machine-specific values:

```bash
cp configs/local.env.example configs/local.env
```

Local files are ignored by git:

```text
configs/local.env
configs/vms.local
```

Use `configs/local.env` for local SSH/tunnel defaults:

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

## Agent layout

```text
agent/
  cmd/agent/              process entrypoint
  ebpf/
    cmd/flow_tracker/     kernel-side eBPF entrypoint
    features/             kernel-side packet/traffic/protocol helpers
    shared/bpf/           kernel-side shared flow structs/maps
    include/              fallback headers used by release builds
  internal/
    exporter/             backend payloads and HTTP sender
    features/             userspace traffic/protocol/classification modules
    flow/                 flow key, state, accumulator and debug formatter
    pipeline/             event dispatcher scaffolding
    config/               env-based agent config
    identity/             VM hostname, machine-id, interface discovery
    lifecycle/            heartbeat / recovery loop
    probe/                connectivity probe server/client
```

The target refactor structure and current-to-target migration map are documented
in [Agent Refactor Structure](agent-refactor-structure.md).

The request/debug path from Docker Compose to VM traffic and frontend rendering
is documented in [Debug Request Flow](runbooks/debug-request-flow.md).

Fresh source download and `FLOW_DEBUG=true|false` setup are documented in
[From Scratch Source Download](runbooks/from-scratch-source-download.md).

## Backend layout

```text
backend/
  cmd/api/                API process entrypoint
  internal/api/           HTTP handlers, routes and middleware
  internal/config/        control-plane config
  internal/db/            embedded PostgreSQL migrations
  internal/model/         API/domain DTOs
  internal/realtime/      SSE hub
  internal/service/       graph, flow, VM, agent and stats services
```

## Frontend layout

```text
frontend/
  src/api/                REST/SSE clients
  src/components/         dashboard components
  src/styles/             CSS
  src/types/              TypeScript DTOs
```

Legacy prototype code has been removed from the active tree. New work should go
into the active root folders above.
