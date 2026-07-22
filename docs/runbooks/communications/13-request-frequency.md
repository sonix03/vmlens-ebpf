# Request frequency

```bash
# VM A -> VM B, banyak koneksi kecil
for i in $(seq 1 100); do curl -s -o /dev/null http://10.20.20.199:8081/; done
```

```bash
# VM B -> VM A, banyak koneksi kecil
for i in $(seq 1 100); do curl -s -o /dev/null http://10.20.20.130:8081/; done
```

```bash
# VM A, request kecil terus berjalan
while true; do curl -s -o /dev/null http://10.20.20.199:8081/; sleep 0.2; done
```

```bash
# Stop loop
Ctrl+C
```

```bash
# Local check
curl -s http://127.0.0.1:8080/api/stats/summary
curl -s 'http://127.0.0.1:8080/api/internal/activity?limit=20'
```
