# VMLens OpenStack Customization Script

Upload this file when creating an OpenStack instance:

```text
configuration/openstack-vmlens-cloud-init.yaml
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

Editable values inside the YAML:

```text
REPO_URL
BACKEND_URL
FLOW_INTERVAL
AGENT_IGNORE_IPS
```
