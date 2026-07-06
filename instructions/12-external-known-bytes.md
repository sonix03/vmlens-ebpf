# External known-byte download

```bash
IFACE=$(ip route show default | awk 'NR==1 {print $5}')
RX_BEFORE=$(cat "/sys/class/net/${IFACE}/statistics/rx_bytes")
curl -4 --fail --show-error --location --output /tmp/vmlens-25MiB.bin --write-out 'status=%{http_code} payload=%{size_download}\n' 'https://speed.cloudflare.com/__down?bytes=26214400'
RX_AFTER=$(cat "/sys/class/net/${IFACE}/statistics/rx_bytes")
printf 'interface_received=%s file_bytes=%s\n' "$((RX_AFTER-RX_BEFORE))" "$(stat -c %s /tmp/vmlens-25MiB.bin)"
```

```bash
rm -f /tmp/vmlens-25MiB.bin
```
