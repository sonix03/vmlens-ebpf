# Redis VM ke VM

```bash
# Redis server VM
sudo ss -ltnp 'sport = :6379'
sudo ufw allow from 10.20.20.130 to any port 6379 proto tcp
```

```bash
# Client VM
redis-cli -h 10.20.20.199 -p 6379 PING
```

```bash
# Cleanup firewall server VM
sudo ufw delete allow from 10.20.20.130 to any port 6379 proto tcp
```
