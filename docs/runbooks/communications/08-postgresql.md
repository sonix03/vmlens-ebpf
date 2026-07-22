# PostgreSQL VM ke VM

```bash
# PostgreSQL server VM
sudo ss -ltnp 'sport = :5432'
sudo ufw allow from 10.20.20.130 to any port 5432 proto tcp
```

```bash
# Client VM
PGPASSWORD='PASSWORD' psql -h 10.20.20.199 -p 5432 -U USERNAME -d DATABASE -c 'SELECT now();'
```

```bash
# Cleanup firewall server VM
sudo ufw delete allow from 10.20.20.130 to any port 5432 proto tcp
```
