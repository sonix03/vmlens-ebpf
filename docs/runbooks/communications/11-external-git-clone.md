# External Git clone

```bash
IFACE=$(ip route show default | awk 'NR==1 {print $5}')
RX_BEFORE=$(cat "/sys/class/net/${IFACE}/statistics/rx_bytes")
TX_BEFORE=$(cat "/sys/class/net/${IFACE}/statistics/tx_bytes")
rm -rf /tmp/vmlens-git-test
git clone --depth 1 --progress https://github.com/git/git.git /tmp/vmlens-git-test
RX_AFTER=$(cat "/sys/class/net/${IFACE}/statistics/rx_bytes")
TX_AFTER=$(cat "/sys/class/net/${IFACE}/statistics/tx_bytes")
printf 'interface=%s received=%s sent=%s\n' "$IFACE" "$((RX_AFTER-RX_BEFORE))" "$((TX_AFTER-TX_BEFORE))"
du -sh /tmp/vmlens-git-test
```

```bash
rm -rf /tmp/vmlens-git-test
```
