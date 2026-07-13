# Legacy v1 stack

This folder keeps the earlier VMLens prototype stack out of the active root
layout.

Active runtime code is now in:

```text
agent/
backend/
frontend/
scripts/
configuration/
```

Legacy contents:

```text
bpf/       older CO-RE programs
cmd/       older CLI entrypoints
config/    older YAML config
deploy/    older monitoring/deployment files
examples/  old traffic/resource demo scripts
internal/  old private Go packages
pkg/       old public-ish Go packages
scripts/   old install/run helpers
```

Use this folder only for historical reference or migration work.
