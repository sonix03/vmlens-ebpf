# AGENTS.md

Guide for AI agents and new contributors working in this repository. This is the
canonical file; `CLAUDE.md` points here and adds Claude Code specifics.

## What VMLens is

VMLens tracks **network relationships between VMs**. A TC/eBPF agent runs on each
VM, observes real packets on the NIC, and reports who talked to whom and how well
that path performed. A control plane aggregates the reports and a dashboard draws
the result as a topology graph.

The product question it answers is: *"is VM A actually connected to VM B, and is
that path healthy?"* — with observed evidence, not configuration. It is built to
sit behind a VM-provider console.

VMLens never captures payloads: no HTTP bodies, TLS plaintext, SSH content,
database queries, files, or command lines.

```text
Cloud VM A ─┐
            │  TC/eBPF flow metadata
Cloud VM B ─┼── vmlens-agent ── reverse SSH tunnel ── control plane ── dashboard
            │
External IP ┘
```

## Read this before touching metrics

Four rules cause almost every misunderstanding in this codebase. The full
contract lives in `contracts/`; these are the ones you cannot skip.

1. **Every flow is written from the observing VM's point of view.** `src_ip` is
   always the VM running the agent — *not* the source IP in the packet header.
   eBPF normalizes this before emitting (`fill_directional_tuple()` for TC,
   `socket_metadata()` for kprobes). `direction` tells you which way the bytes
   moved.

2. **`dst_port` is not always the service port.** On a server-side flow it is the
   client's ephemeral port. Use the backend-resolved `service_port` for anything
   shown to a user.

3. **Ingest counters are per-window deltas; read-back counters are cumulative.**
   The agent drains and posts every `FLOW_INTERVAL` (default 2s). Never mix the
   two in one calculation.

4. **Keep the four evidence sources separate**, in code and in UI:

   ```text
   eBPF tells what happened.
   Probe tells what currently works.
   Inventory tells what we expected.
   Cloud API tells what was configured.
   ```

`contracts/` is the source of truth for data semantics:

| File | Scope |
| --- | --- |
| `contracts/README.md` | Index and the rules above |
| `contracts/metrics.md` | What is measured and what each number may mean |
| `contracts/telemetry-schema.md` | Exact JSON per hop, plus consumer stability rules |
| `contracts/probing.md` | Active connectivity probing |
| `contracts/inventory-cloud-ui.md` | Inventory format, cloud data, UI state mapping |
| `contracts/known-gaps.md` | Where the implementation is approximate, and what is dead code |

**If you change metric semantics, update the contract in the same change.** A
number whose meaning drifts from its contract is worse than a missing number.

## Layout

```text
agent/      Go module — TC/eBPF collector, reducers, exporter
  internal/features/    one directory per signal (traffic, protocols, classification)
  internal/flow/        aggregation buckets (Key, State, Accumulator)
  internal/pipeline/    FlowMetric, the internal agent contract
  internal/exporter/    backend payloads
  internal/shared/bpf/  shared C headers
backend/    Go module — API, services, Postgres
  internal/service/     flow, graph, classifier, inventory
  internal/telemetry/   validation and derived metrics
  internal/db/migrations/
frontend/   Vite + React + TypeScript dashboard
contracts/  data contract (see above)
docs/       architecture, runbooks, setup guides
scripts/    stack, tunnel, agent install, traffic generation
configs/    vms.local inventory (gitignored), systemd units
dist/agent/ prebuilt agent release artifacts
```

`agent/` and `backend/` are **separate Go modules** joined by `go.work`. Code
cannot be shared between them without introducing a third module — this is why
some logic (e.g. request inference) exists in both.

## Commands

```bash
make build            # build both Go modules
make test             # go test ./... in both modules
make up               # docker compose up -d --build
make down
make logs

cd agent && go test ./...
cd backend && go test ./...

npm --prefix frontend run build    # tsc -b && vite build
npm --prefix frontend run dev
```

Local stack: dashboard `:3000`, API `:8080`, Postgres `:5432`. Compose services
are named `dashboard`, `control-plane`, `datastore`.

**Node version**: the Vite 5 build needs Node 20+. On Node 18 it dies with a
`SyntaxError` inside Vite's own dist chunks, which looks like a code error but is
not. Check `node --version` first.

The dashboard container builds the frontend at image build time. After changing
frontend source you must `docker compose up -d --build dashboard` for the running
dashboard to pick it up — rebuilding only with `npm run build` is not enough.

## Testing against real VMs

The lab is three VMs defined in `configs/vms.local` (gitignored). Full procedure:
`docs/vmlens-three-vm-traffic-test.md`.

```bash
bash scripts/vmlens-stack.sh start
bash scripts/vmlens-tunnel.sh list
bash scripts/vmlens-tunnel.sh start-all      # or: start <alias>
bash scripts/vmlens-tunnel.sh status <alias>
```

Use `scripts/vmlens-tunnel.sh` rather than hand-rolled `ssh -R`. It reads the
inventory, resolves per-VM `proxy_jump`, discovers the SSH key (including under
`/mnt/c/Users/*/.ssh/` on WSL), and tracks tunnel state.

Environment quirks worth knowing before you diagnose a "connectivity problem":

- Agents post to `127.0.0.1:18080` on their own host, which the reverse tunnel
  maps to the local backend. Without a tunnel they log `connection refused` and
  retry forever — that is normal, not a failure.
- Not every VM needs its own tunnel. A VM can reach the control plane through a
  relay on a bastion (this lab runs `vmlens-backend-relay-130.service` on the
  bastion so the third VM rides its tunnel).
- On WSL, **ICMP to the lab subnet fails while TCP works.** `ping` is not a valid
  reachability test here; test the actual port instead.

## Conventions

- Match the density of surrounding code. Comments are sparse and explain *why*,
  not *what*.
- Docs and contracts are written in English, in prose with tables — not bullet
  soup. Existing files set the tone.
- `frontend/src/styles/index.css` has an old compressed section at the top and
  newer expanded sections appended below. Later rules win by cascade order; add
  new blocks at the end rather than editing the compressed lines.
- eBPF C lives next to the Go feature that owns it and compiles into one object,
  `flow_tracker.bpf.o`. Rebuilding it needs clang/bpftool, which are not always
  present — check before assuming you can rebuild.
- Prebuilt agents in `dist/agent/<tag>/` may lag the source tree. Verify what a
  deployed binary actually contains (`strings … | grep trace_tcp_rtt`) before
  concluding a metric is broken; the feature may simply not be deployed.

## Landmines

- A permanently zero column usually means the producer was never wired up, not
  that the path is idle. `avg_app_delay_ms` is the current example.
- `error_count` counts every TCP RST, including healthy RST-based close. Non-zero
  errors do not mean the path is broken.
- `request_count` for TCP equals connection attempts, not requests.
- Graph edges merge all ports for a VM pair, so a failure on one port colours the
  whole relationship.

These and others are tracked with evidence in `contracts/known-gaps.md`. Read it
before "fixing" a number that looks wrong — it may be a documented approximation.
