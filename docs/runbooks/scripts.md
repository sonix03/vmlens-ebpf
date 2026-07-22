# VMLens Cloud VM Setup

Runbook ini memakai contoh VM yang sudah berhasil dipakai sebelumnya:

- SSH host: `ubuntu@10.20.20.103`
- SSH key Windows: `%USERPROFILE%\.ssh\id_ed25519_vmlens`
- IP VM: dideteksi otomatis dari interface yang memiliki default route
- backend laptop: `127.0.0.1:8080`
- backend dari sisi VM melalui reverse tunnel: `127.0.0.1:18080`

Ganti nilai IP bila memasang agent pada VM lain.

## 1. Pastikan VMLens lokal berjalan

Jalankan di PowerShell Windows pada laptop:

```powershell
cd C:\Documents\Ionext\vmlens-ebpf
docker compose up -d --build
Invoke-RestMethod http://127.0.0.1:8080/health
```

Respons health harus menunjukkan status `ok` sebelum melanjutkan.

## 2. Buka SSH dan reverse tunnel

Jalankan di PowerShell Windows:

```powershell
ssh -i "$env:USERPROFILE\.ssh\id_ed25519_vmlens" -o IdentitiesOnly=yes -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 -R 127.0.0.1:18080:127.0.0.1:8080 ubuntu@10.20.20.103
```

Jangan tutup sesi SSH ini. Agent pada VM memakai tunnel tersebut untuk mencapai
backend yang berjalan di laptop.

## 3. Install atau update agent pada VM

Setelah prompt berubah menjadi `ubuntu@...`, salin seluruh blok berikut ke VM.
Setup dijalankan dalam child shell. Jika satu langkah gagal, prompt SSH tetap
terbuka dan tunnel tidak otomatis ikut tertutup.

```bash
bash <<'VMLENS_SETUP'
set -Eeuo pipefail

REPO_URL="https://github.com/sonix03/vmlens-ebpf.git"
REPO_DIR="$HOME/vmlens-ebpf"
BACKEND_URL="http://127.0.0.1:18080"
SSH_PEER_IP="${SSH_CONNECTION%% *}"

sudo apt-get update
sudo apt-get install -y git curl ca-certificates golang-go clang libbpf-dev

# Ubuntu menyediakan bpftool melalui paket linux-tools, bukan selalu melalui
# paket bernama bpftool.
if ! sudo apt-get install -y linux-tools-common "linux-tools-$(uname -r)"; then
  sudo apt-get install -y linux-tools-common linux-tools-generic
fi

if ! bpftool version >/dev/null 2>&1; then
  BPFTOOL_PATH="$(find /usr/lib/linux-tools -name bpftool -type f 2>/dev/null | sort -V | tail -n1 || true)"
  if [[ -z "$BPFTOOL_PATH" ]]; then
    echo "ERROR: bpftool tidak ditemukan setelah instalasi linux-tools" >&2
    exit 1
  fi
  sudo ln -sf "$BPFTOOL_PATH" /usr/local/sbin/bpftool
  hash -r
fi

bpftool version
go version
test -r /sys/kernel/btf/vmlinux

echo "Default route dan source IP yang akan dideteksi agent:"
ip -o -4 route show default
ip -o -4 route get 1.1.1.1

if [[ -d "$REPO_DIR/.git" ]]; then
  git -C "$REPO_DIR" pull --ff-only
else
  git clone "$REPO_URL" "$REPO_DIR"
fi

curl -fsS "$BACKEND_URL/health"
echo

cd "$REPO_DIR"
sudo env \
  BACKEND_URL="$BACKEND_URL" \
  MOCK_MODE=false \
  AGENT_IGNORE_IPS="$SSH_PEER_IP" \
  AGENT_ENVIRONMENT=cloud-vm \
  bash scripts/install-agent.sh

sudo systemctl --no-pager --full status vmlens-agent
sudo journalctl -u vmlens-agent -n 30 --no-pager
VMLENS_SETUP
```

VM akan muncul sebagai node di `http://localhost:3000` setelah register dan
heartbeat berhasil.

`AGENT_PRIVATE_IPS` sengaja tidak diberikan. Agent membaca interface aktif yang
memiliki default route dan mendaftarkan IPv4/MAC interface tersebut. Override
manual hanya diperlukan jika alamat tujuan memakai IP NAT/public yang tidak
terpasang pada interface VM:

```bash
AGENT_PRIVATE_IPS="IP_INTERFACE,IP_NAT_TAMBAHAN"
```

`AGENT_IGNORE_IPS` diisi otomatis dengan IP peer SSH agar traffic reverse
tunnel VMLens tidak dikoleksi kembali dan membuat feedback loop.

## 4. Test flow sederhana

Jalankan pada VM:

```bash
curl -fsSL https://example.com -o /dev/null
sudo journalctl -u vmlens-agent -n 30 --no-pager
```

Cek hasil dari PowerShell laptop:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/api/vms
Invoke-RestMethod http://127.0.0.1:8080/api/flows
```

## 5. Test garis komunikasi antar-VM

Agent harus sudah terpasang pada VM A dan VM B. IP tujuan yang dipakai VM A
harus termasuk dalam IP yang didaftarkan oleh agent VM B.

Pada VM B:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

Izinkan TCP `8081` hanya dari private IP VM A melalui firewall/security group.
Kemudian pada VM A:

```bash
cd "$HOME/vmlens-ebpf"
bash scripts/test-vm-communication.sh VM_B_PRIVATE_IP 8081
```

Buka `http://localhost:3000`. Setelah flow diterima backend, topology akan
menampilkan garis berarah VM A → VM B. Untuk menguji arah sebaliknya, jalankan
server pada VM A dan script traffic pada VM B.

Jika garis belum muncul, cek dari laptop:

```powershell
Invoke-RestMethod http://127.0.0.1:8080/api/vms
Invoke-RestMethod http://127.0.0.1:8080/api/flows
Invoke-RestMethod "http://127.0.0.1:8080/api/graph?time_range=5m"
```

## 6. Perilaku ketika VM dihapus

Ketika heartbeat berhenti, node tetap menjadi inventory. Warnanya berubah dari
terang menjadi `stale`, lalu `offline` setelah 5 menit. Timeout tidak menghapus
record secara default.

```env
VM_DELETE_AFTER=0
```

Backend tidak dapat membedakan VM yang dihapus dengan VM yang mati atau putus
jaringan tanpa API OpenStack. Penghapusan node saat instance dihapus akan
memerlukan integrasi lifecycle OpenStack; sementara itu node dipertahankan
sebagai offline inventory.

## Troubleshooting cepat

### Menghapus seluruh node/data test lokal

Volume PostgreSQL menyimpan node meskipun container di-rebuild. Untuk reset
development hingga topology benar-benar kosong, jalankan di PowerShell laptop:

```powershell
cd C:\Documents\Ionext\vmlens-ebpf
docker compose down -v
docker compose up -d --build --force-recreate
Invoke-RestMethod http://127.0.0.1:8080/api/graph
```

Perintah `down -v` menghapus seluruh history VM dan flow lokal. Jangan gunakan
pada deployment yang datanya perlu dipertahankan.

### `Package 'bpftool' has no installation candidate`

Jangan install paket virtual `bpftool` secara langsung. Gunakan:

```bash
sudo apt-get install -y linux-tools-common "linux-tools-$(uname -r)"
```

Blok setup di atas sudah menangani fallback ke `linux-tools-generic` dan mencari
binary `bpftool` yang terpasang.

### Health tunnel gagal

Pada VM:

```bash
curl -v http://127.0.0.1:18080/health
```

Jika gagal, pastikan backend lokal aktif dan sesi PowerShell yang membuat
reverse tunnel belum ditutup.

### Agent gagal start

```bash
sudo systemctl --no-pager --full status vmlens-agent
sudo journalctl -u vmlens-agent -n 100 --no-pager
```
