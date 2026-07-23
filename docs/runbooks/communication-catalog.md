# Instruksi komunikasi VM

Dokumen ini menjelaskan komunikasi yang dapat divisualisasikan VMLens, cara
menguji VM A ke VM B, dan batasan teknisnya.

## Perilaku garis live

VMLens menampilkan satu garis relasi untuk sepasang VM yang sudah terdaftar.

- Garis hijau terang dan bergerak berarti ada aktivitas TCP/UDP terbaru.
- Selama paket/data berikutnya terus terdeteksi, garis terus bergerak.
- Setelah tidak ada aktivitas selama `FLOW_ACTIVE_WINDOW` (default `4s`),
  animasi berhenti dan garis menjadi abu-abu redup.
- Garis redup tetap disimpan sebagai histori relasi dalam jendela graph 24 jam.
- Event baru pada relasi yang sama langsung mengaktifkan animasinya kembali.
- Event `flow.updated` diterapkan langsung di browser melalui SSE; graph tidak
  menunggu polling atau reload halaman untuk menyalakan garis.

Window tiga detik membuat request yang sangat singkat tetap terlihat oleh
manusia. Nilainya dapat diubah melalui `.env`:

```dotenv
FLOW_ACTIVE_WINDOW=4s
```

Nilai yang diterima adalah `500ms` sampai `1m`. Setelah mengubahnya, rebuild
backend:

```bash
docker compose up -d --build backend frontend
```

Status ini berdasarkan aktivitas jaringan terakhir. VMLens tidak membaca isi
request. Karena itu, batas request aplikasi tidak selalu sama dengan batas
koneksi jaringan. HTTPS, HTTP/2, WebSocket, gRPC, dan connection pool dapat
memakai satu koneksi untuk banyak request.

## Komunikasi yang didukung collector saat ini

Collector eBPF saat ini mengamati metadata **IPv4 TCP dan UDP**. Semua protokol
aplikasi yang berjalan di atas keduanya dapat menghasilkan garis.

Graph utama hanya menggambar garis jika kedua endpoint terdaftar sebagai VM.
Trafik menuju komputer local, IP internet publik, atau host internal tanpa agent
tetap disimpan backend dan tersedia melalui API, tetapi tidak digambar pada
tampilan utama agar topology tetap fokus pada hubungan antar-VM.

| Komunikasi | Transport umum | Port umum | Contoh penggunaan |
|---|---|---:|---|
| HTTP frontend/backend atau REST API | TCP | 80, 8080, 3000 | Browser/Frontend ke backend API |
| HTTPS | TCP | 443 | Web/API terenkripsi |
| WebSocket | TCP | 80, 443 | Chat, notification, live dashboard |
| gRPC | TCP/HTTP2 | 443, 50051 | Komunikasi antarmicroservice |
| SSH, SCP, SFTP | TCP | 22 | Remote shell dan transfer file |
| PostgreSQL | TCP | 5432 | Backend ke database |
| MySQL/MariaDB | TCP | 3306 | Backend ke database |
| Redis | TCP | 6379 | Cache/session/pub-sub |
| MongoDB | TCP | 27017 | Database dokumen |
| Kafka | TCP | 9092 | Event streaming |
| RabbitMQ/AMQP | TCP | 5672, 5671 | Message queue |
| DNS | UDP atau TCP | 53 | Resolusi nama domain |
| NTP | UDP | 123 | Sinkronisasi waktu |
| Syslog | UDP atau TCP | 514 | Pengiriman log |
| SMTP | TCP | 25, 465, 587 | Pengiriman email |
| NFS | TCP/UDP | 2049 | Shared filesystem Linux |
| SMB | TCP | 445 | File sharing Windows/Samba |
| Custom application | TCP/UDP | bebas | Service buatan sendiri |

Nomor port membantu mengidentifikasi kemungkinan service, tetapi bukan bukti
mutlak. Aplikasi dapat memakai port apa pun dan VMLens tidak memeriksa payload.

## Contoh frontend dan backend

Untuk frontend React/Vue/Angular yang dijalankan browser:

```text
Browser pengguna --HTTPS--> Backend/API di VM B --TCP--> Database di VM C
```

Request `fetch` atau Axios dilakukan oleh browser pengguna. Jika frontend hanya
berupa file statis di VM A, garis A ke B belum tentu muncul; yang menghubungi B
adalah browser pengguna. Garis A ke B muncul bila proses di VM A sendiri, seperti
Nginx reverse proxy atau server-side rendering, meneruskan request ke VM B:

```text
Browser --HTTPS--> Nginx VM A --HTTP/gRPC--> Backend VM B
```

## Persiapan VM A dan VM B

1. Install dan jalankan `vmlens-agent` pada kedua VM.
2. Gunakan `TENANT_ID` yang sama jika relasi harus diklasifikasikan
   `internal_same_tenant`.
3. Pastikan private IP tujuan yang dipakai VM A juga diregistrasikan agent VM B.
4. Buka hanya port pengujian yang diperlukan pada firewall/security group.
5. Hindari membuka database, Redis, atau service uji ke internet publik.

Contoh install dari repository di masing-masing VM:

```bash
sudo env BACKEND_URL=http://IP_BACKEND_VMLENS:8080 \
  TENANT_ID=tenant-demo \
  MOCK_MODE=false \
  bash scripts/install-agent.sh
```

## Pengujian komunikasi

Semua contoh di bawah harus dijalankan **dari VM A menuju private IP VM B**.
Menjalankannya dari laptop operator tidak membuat garis VM A-B karena laptop
bukan node VM terdaftar. Gunakan pengujian berbatas agar proses berhenti sendiri.

### HTTP

Di VM B:

```bash
python3 -m http.server 8081 --bind 0.0.0.0
```

Di VM A:

```bash
curl http://IP_PRIVATE_VM_B:8081/
```

Garis A-B bergerak saat event HTTP terdeteksi, kemudian menjadi statis setelah
window aktivitas habis.

### HTTP tanpa curl

Dengan `wget` dari VM A:

```bash
for i in $(seq 1 20); do
  wget -q -O /dev/null http://IP_PRIVATE_VM_B:8081/
  sleep 0.5
done
```

Atau memakai standard library Python, tanpa dependency tambahan:

```bash
python3 -c "import urllib.request; print(urllib.request.urlopen('http://IP_PRIVATE_VM_B:8081/', timeout=5).status)"
```

### TCP tanpa HTTP

Di VM B:

```bash
nc -lk 9000
```

Kirim 20 koneksi TCP dari VM A:

```bash
for i in $(seq 1 20); do
  printf 'tcp-message-%s\n' "$i" | nc -q 1 IP_PRIVATE_VM_B 9000
  sleep 0.5
done
```

VMLens melihat metadata dan jumlah byte, tetapi tidak merekam isi pesannya.

### UDP

Di VM B:

```bash
nc -u -lk 9001
```

Kirim datagram dari VM A:

```bash
for i in $(seq 1 20); do
  printf 'udp-message-%s\n' "$i" | nc -u -w 1 IP_PRIVATE_VM_B 9001
  sleep 0.5
done
```

UDP tidak mempunyai lifecycle koneksi seperti TCP. Garis aktif ketika datagram
terdeteksi dan berhenti berdasarkan inactivity window.

### SSH dan transfer file

Di VM A:

```bash
ssh user@IP_PRIVATE_VM_B 'hostname && uptime'
scp ./contoh.txt user@IP_PRIVATE_VM_B:/tmp/
```

Keduanya terlihat sebagai TCP port 22. Isi terminal dan file tidak direkam.

### Database dan cache

Contoh client dari VM A menuju service di VM B:

```bash
psql -h IP_PRIVATE_VM_B -p 5432 -U app appdb
redis-cli -h IP_PRIVATE_VM_B -p 6379 PING
```

Jalankan hanya jika PostgreSQL/Redis memang dikonfigurasi menerima koneksi dari
VM A.

### DNS dan trafik keluar VM

Contoh ini menghasilkan metrik external karena tujuannya bukan VM terdaftar:

```bash
for i in $(seq 1 10); do
  dig @8.8.8.8 example.com +short
  sleep 1
done
```

Node internet tidak ditampilkan pada topology VM-only, tetapi byte dan jumlah
flow-nya masuk ke kartu `External traffic`.

## Metrik trafik

Halaman utama menampilkan metrik kumulatif berikut dan memperbaruinya selama
event real-time masuk:

- `Internal VM traffic`: total byte TCP/UDP dengan scope
  `internal_same_tenant` atau `internal_cross_tenant`.
- `External traffic`: total byte menuju alamat public dengan scope
  `external_public`.
- `Request frequency`: estimasi request/koneksi network-level dari eBPF.
  TCP dihitung dari koneksi baru, UDP dihitung dari datagram lokal.
- Panah `↑` adalah `bytes_sent`, panah `↓` adalah `bytes_received`.
- Jumlah `flows` adalah relasi agregat berdasarkan VM/IP, protokol, dan port;
  bukan jumlah request HTTP.
- Nilainya kumulatif selama data masih tersimpan di PostgreSQL, bukan rate per
  detik. Karena kedua agent dapat melihat pertukaran yang sama dari sisi
  masing-masing, gunakan angka ini untuk observability/topology, bukan billing
  byte jaringan.

Endpoint mentah metrik:

```bash
curl http://127.0.0.1:8080/api/stats/summary
curl 'http://127.0.0.1:8080/api/internal/activity?limit=20'
```

Command request kecil untuk menguji frekuensi:

```bash
for i in $(seq 1 100); do curl -s -o /dev/null http://IP_PRIVATE_VM_B:8081/; done
while true; do curl -s -o /dev/null http://IP_PRIVATE_VM_B:8081/; sleep 0.2; done
```

## Komunikasi yang belum divisualisasikan

- IPv6.
- ICMP seperti `ping` dan `traceroute` berbasis ICMP.
- Unix domain socket dan komunikasi antarproses yang tidak melewati jaringan.
- Loopback `127.0.0.1`; agent sengaja mengabaikannya.
- Koneksi agent menuju backend VMLens; control-plane sengaja dikecualikan agar
  tidak membuat feedback loop.
- Payload, URL, HTTP method/status, header/body, query database, isi SSH, dan isi
  file.
- Batas request aplikasi yang presisi pada koneksi persistent/multiplexed.

Untuk lifecycle request HTTP/gRPC yang presisi, korelasikan VMLens dengan
OpenTelemetry pada frontend server/backend. VMLens tetap bertugas menunjukkan
relasi jaringan VM, sedangkan tracing aplikasi menunjukkan span request.
