# VMLens OpenStack Customization Script

Use one of these files when creating an OpenStack instance.

## Option A: build on the VM

Use this when the VM is allowed to install Go, clang, libbpf-dev, and compile
the agent during boot:

```text
configuration/openstack-vmlens-cloud-init.yaml
```

## Option B: install prebuilt release artifacts

Use this when GitHub Release already has precompiled assets:

```text
configuration/openstack-vmlens-prebuilt-cloud-init.yaml
```

Required release assets:

```text
vmlens-agent-linux-amd64
flow_tracker-linux-amd64.bpf.o
install-agent.sh
SHA256SUMS
```

OpenStack field:

```text
Customization Script -> Load Customization Script from file
```

After the VM is created, start the reverse tunnel from local:

```bash
bash scripts/vmlens-tunnel.sh start <VM_IP_OR_ALIAS>
```

The agent inside the VM uses:

```text
http://127.0.0.1:18080
```

If the tunnel is not ready yet, the agent will keep retrying.

Check bootstrap logs inside the VM:

```bash
sudo tail -n 200 /var/log/cloud-init-output.log
sudo tail -n 200 /var/log/vmlens-bootstrap.log
```

Run bootstrap manually again if needed:

```bash
sudo bash /usr/local/sbin/vmlens-bootstrap.sh
```

For prebuilt mode:

```bash
sudo bash /usr/local/sbin/vmlens-bootstrap-prebuilt.sh
```

Editable values inside the YAML:

```text
REPO_URL
RELEASE_BASE_URL
BACKEND_URL
FLOW_INTERVAL
CAPTURE_MODE
CAPTURE_INTERFACE
AGENT_IGNORE_IPS
FLOW_ALLOW_CIDRS
FLOW_DENY_CIDRS
```

Recommended OpenStack network capture values:

```text
CAPTURE_MODE=tc
CAPTURE_INTERFACE=ens3
```

If `ens3` does not exist, the provided cloud-init scripts automatically fall
back to the VM default-route interface.
