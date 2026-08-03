# CLAUDE.md

**Read `AGENTS.md` first.** It is the canonical guide: what VMLens is, the repo
layout, build and test commands, the data-contract rules, and the VM testing
workflow. This file only adds what is specific to working here through Claude
Code.

## Before claiming work is done

Both Go modules and the frontend must be green:

```bash
cd agent && go build ./... && go test ./...
cd backend && go build ./... && go test ./...
npm --prefix frontend run build
```

Frontend changes are not visible in the running dashboard until the container is
rebuilt: `docker compose up -d --build dashboard`. Verify by fetching the served
asset and grepping for the class or label you added, rather than assuming.

## Do not do these without asking

- Installing, upgrading, or restarting the agent on a lab VM. That is software
  deployment on someone's infrastructure.
- Deleting files or freeing disk on a VM beyond caches and rotated logs.
- Anything that changes a VM's network state, such as `tc qdisc` / `netem`.

Reading state over SSH, starting tunnels via `scripts/vmlens-tunnel.sh`, and
generating test traffic from the runbook are fine.

## Environment traps that have already cost time

- **`ping` lies here.** WSL cannot send ICMP to the lab subnet but TCP works
  fine. Test the port (`/dev/tcp/<ip>/22`), never conclude "unreachable" from a
  failed ping.
- **Long-lived `ssh -R` is blocked** by the permission classifier. Use
  `scripts/vmlens-tunnel.sh start-all`, which is also the project's intended
  path.
- **Node 18 cannot build the frontend.** The Vite failure looks like a syntax
  error in application code but is a Node version problem. Use Node 20+.
- **A deployed agent may be older than the source tree.** Before reporting a
  metric as broken, confirm the running binary actually contains the feature.

## When changing metrics

`contracts/` is the source of truth for what every number means. Update the
relevant contract in the same change as the code — the semantics and the
documentation must not drift apart. New findings about existing behaviour belong
in `contracts/known-gaps.md`, with the evidence that produced them.

Distinguish clearly between a finding **read from code** and one **verified
against the lab**. `known-gaps.md` marks this per entry, and it matters: a
code-reading conclusion can be wrong about what the deployed system does.
