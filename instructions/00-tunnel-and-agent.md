# Tunnel dan agent

```powershell
# Local PowerShell - terminal 1
ssh -N -i "$env:USERPROFILE\.ssh\id_ed25519_vmlens" -o IdentitiesOnly=yes -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -R 127.0.0.1:18080:127.0.0.1:8080 ubuntu@10.20.20.130
```

```powershell
# Local PowerShell - terminal 2
ssh -N -i "$env:USERPROFILE\.ssh\id_ed25519_vmlens" -o IdentitiesOnly=yes -o ExitOnForwardFailure=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -R 127.0.0.1:18080:127.0.0.1:8080 ubuntu@10.20.20.199
```

```bash
# Jalankan di kedua VM
curl http://127.0.0.1:18080/health
sudo systemctl restart vmlens-agent
sudo systemctl status vmlens-agent --no-pager
sudo journalctl -u vmlens-agent -n 30 --no-pager
```
