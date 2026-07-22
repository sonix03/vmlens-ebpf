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
legacy/         old v1 prototype stack, kept for reference
```

## Deploy layout

```text
deploy/
  deepflow/      local DeepFlow service config used by docker-compose.deepflow.yml
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
    programs/             kernel-side eBPF C programs
    include/              fallback headers used by release builds
    README.md             eBPF build notes
  internal/
    capture/              mock, kprobe and TCX capture collectors
    config/               env-based agent config
    identity/             VM hostname, machine-id, interface discovery
    lifecycle/            heartbeat / recovery loop
    telemetry/            JSON payload types sent to backend
    transport/            HTTP sender to control-plane
```

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

## Legacy layout

```text
legacy/v1-stack/
  bpf/       old CO-RE programs
  cmd/       old CLI entrypoints
  config/    old YAML config
  deploy/    old monitoring/deployment files
  examples/  old traffic/resource demos
  internal/  old private Go packages
  pkg/       old public-ish Go packages
  scripts/   old install/run helpers
```

Use `legacy/v1-stack/` only for reference or migration work. New work should go
into the active root folders above.
