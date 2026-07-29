# From Scratch Source Download

Runbook ini dipakai saat ingin mulai lagi dari source yang benar-benar bersih.
Source lama tidak dihapus permanen; script akan memindahkannya ke folder backup
timestamp.

## 1. Local dashboard dari source bersih

```bash
export VMLENS_SOURCE_DIR="$HOME/vmlens-ebpf"
export VMLENS_REPO_URL="https://github.com/sonix03/vmlens-ebpf.git"
export FRESH_SOURCE=true

bash scripts/download-source.sh
cd "$VMLENS_SOURCE_DIR"

docker compose up -d --build
curl -fsS http://127.0.0.1:8080/health
```

Dashboard:

```text
http://localhost:3000
```

## 2. VM source install dengan FLOW_DEBUG=false

Pakai ini untuk mode normal.

```bash
export VMLENS_SOURCE_DIR="$HOME/vmlens-ebpf"
export VMLENS_REPO_URL="https://github.com/sonix03/vmlens-ebpf.git"
export FRESH_SOURCE=true

curl -fsSL -o /tmp/vmlens-download-source.sh \
  https://raw.githubusercontent.com/sonix03/vmlens-ebpf/main/scripts/download-source.sh

bash /tmp/vmlens-download-source.sh
cd "$VMLENS_SOURCE_DIR"

sudo env \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  FLOW_INTERVAL=1s \
  FLOW_DEBUG=false \
  CONNECTIVITY_PROBE_ENABLED=true \
  CONNECTIVITY_PROBE_INTERVAL=5s \
  CONNECTIVITY_PROBE_LISTEN_ADDR=0.0.0.0:18081 \
  INSTALL_MODE=build \
  bash scripts/install-agent.sh
```

## 3. VM source install dengan FLOW_DEBUG=true

Pakai ini saat ingin melihat alur:

```text
capture → aggregate → export
```

```bash
sudo env \
  BACKEND_URL=http://127.0.0.1:18080 \
  MOCK_MODE=false \
  CAPTURE_MODE=tc \
  CAPTURE_INTERFACE=ens3 \
  FLOW_INTERVAL=1s \
  FLOW_DEBUG=true \
  CONNECTIVITY_PROBE_ENABLED=true \
  CONNECTIVITY_PROBE_INTERVAL=5s \
  CONNECTIVITY_PROBE_LISTEN_ADDR=0.0.0.0:18081 \
  INSTALL_MODE=build \
  bash scripts/install-agent.sh
```

Lihat debug:

```bash
sudo journalctl -u vmlens-agent -f | grep flow_debug
```

Expected:

```text
flow_debug stage=captured ...
flow_debug stage=drain batch_size=...
flow_debug stage=exported ...
```

## 4. Toggle setelah agent sudah terinstall

```bash
bash scripts/vmlens-agent.sh debug-status
bash scripts/vmlens-agent.sh debug-on
sudo journalctl -u vmlens-agent -f | grep flow_debug
bash scripts/vmlens-agent.sh debug-off
```

Perintah itu mengubah:

```text
/etc/vmlens/agent.env
```

Lalu restart service:

```text
vmlens-agent
```

## 5. Validasi VM

```bash
systemctl is-active vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 80 --no-pager
cat /etc/vmlens/agent.env | grep FLOW_DEBUG
```

## 6. Kembali ke source lama jika perlu

Kalau `FRESH_SOURCE=true`, source lama dipindahkan menjadi:

```text
$VMLENS_SOURCE_DIR.backup.YYYYMMDDHHMMSS
```

Restore manual:

```bash
mv "$VMLENS_SOURCE_DIR" "$VMLENS_SOURCE_DIR.new"
mv "$VMLENS_SOURCE_DIR.backup.<timestamp>" "$VMLENS_SOURCE_DIR"
```
