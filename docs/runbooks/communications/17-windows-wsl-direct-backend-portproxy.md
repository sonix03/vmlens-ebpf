# Windows + WSL direct backend portproxy

Use this only when cloud/test VMs must reach the local backend directly without
reverse SSH tunnel.

Problem:

```bash
curl http://127.0.0.1:8080/health
```

works locally, but this from VM fails:

```bash
curl http://10.20.20.125:8080/health
```

Reason: the backend is reachable inside WSL/Docker, but not exposed on the
Windows LAN IP.

## 1. Get WSL IP

PowerShell:

```powershell
wsl hostname -I
```

Example:

```text
172.28.142.250
```

## 2. Add portproxy

PowerShell as Administrator:

```powershell
netsh interface portproxy add v4tov4 listenaddress=10.20.20.125 listenport=8080 connectaddress=<WSL_IP> connectport=8080
netsh advfirewall firewall add rule name="VMLens backend 8080" dir=in action=allow protocol=TCP localport=8080
netsh interface portproxy show all
```

## 3. Validate from local

PowerShell:

```powershell
curl.exe http://10.20.20.125:8080/health
```

## 4. Validate from each VM

VM:

```bash
curl http://10.20.20.125:8080/health
```

## 5. Start agent without tunnel

VM:

```bash
BACKEND_URL=http://10.20.20.125:8080 \
bash scripts/vmlens-agent.sh start
```

## 6. Remove portproxy

PowerShell as Administrator:

```powershell
netsh interface portproxy delete v4tov4 listenaddress=10.20.20.125 listenport=8080
netsh advfirewall firewall delete rule name="VMLens backend 8080"
```
