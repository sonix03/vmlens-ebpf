# DNS external

```bash
for i in $(seq 1 10); do dig @8.8.8.8 example.com +short; sleep 1; done
```

```bash
sudo apt-get update
sudo apt-get install -y dnsutils
```
