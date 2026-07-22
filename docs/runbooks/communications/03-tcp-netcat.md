# TCP netcat

```bash
# Server VM
nc -lk 9000
```

```bash
# Client VM
SERVER_IP=10.20.20.199
for i in $(seq 1 20); do printf 'tcp-message-%s\n' "$i" | nc -q 1 "$SERVER_IP" 9000; sleep 0.5; done
```

```bash
# Firewall server VM
sudo ufw allow from 10.20.20.130 to any port 9000 proto tcp
sudo ufw delete allow from 10.20.20.130 to any port 9000 proto tcp
```
