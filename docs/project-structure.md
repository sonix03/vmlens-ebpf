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
cp configs/vms.example configs/vms.local
```

Local files are ignored by git:

```text
configs/vms.local
```

Use `configs/vms.local` for the VM list:

```text
alias|host|ssh_user|ssh_key|remote_backend|local_backend|proxy_jump|role|type|environment|owner|tenant_id|project_id|region|zone|network_id|subnet_id|public_ip|provider_id|probe_protocol|probe_port|capture_interface|ignore_ports|ignore_ips|flow_allow_cidrs|flow_deny_cidrs|notes
```

The first seven fields drive the SSH tunnel script. The remaining fields are
backend/cloud metadata applied once when the backend starts or when an agent
registers. Changing this file requires a backend restart to re-apply metadata.

Optional overrides can be passed as environment variables when needed:
`SSH_USER`, `SSH_KEY`, `LOCAL_BACKEND`, `REMOTE_BACKEND`, `VM_INVENTORY`.

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
