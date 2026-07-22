# UDP netcat

```bash
# Server VM
nc -u -lk 9001
```

```bash
# Client VM
SERVER_IP=10.20.20.199
for i in $(seq 1 20); do printf 'udp-message-%s\n' "$i" | nc -u -w 1 "$SERVER_IP" 9001; sleep 0.5; done
```

```bash
# Firewall server VM
sudo ufw allow from 10.20.20.130 to any port 9001 proto udp
sudo ufw delete allow from 10.20.20.130 to any port 9001 proto udp
```
