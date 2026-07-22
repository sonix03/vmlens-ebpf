# Deployment Assets

Runtime and provisioning assets live here.

```text
deepflow/      local DeepFlow services used by docker-compose.deepflow.yml
openstack/     OpenStack cloud-init / Customization Script files
```

Active local development still starts from the repository root:

```bash
docker compose up -d --build
```

Run with bundled DeepFlow services:

```bash
docker compose -f docker-compose.yml -f docker-compose.deepflow.yml up -d --build
```

