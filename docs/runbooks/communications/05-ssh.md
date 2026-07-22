# SSH VM ke VM

```bash
# testing-a-1 ke testing-a-2
ssh ubuntu@10.20.20.199 'hostname && uptime'
```

```bash
# testing-a-2 ke testing-a-1
ssh ubuntu@10.20.20.130 'hostname && uptime'
```
