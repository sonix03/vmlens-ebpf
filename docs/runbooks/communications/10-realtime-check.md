# Realtime check

```powershell
# Local PowerShell
curl.exe http://127.0.0.1:8080/health
curl.exe http://127.0.0.1:8080/api/agents
curl.exe http://127.0.0.1:8080/api/stats/summary
curl.exe "http://127.0.0.1:8080/api/graph?time_range=15m"
```

```powershell
# Local PowerShell - SSE stream
curl.exe -N http://127.0.0.1:8080/api/realtime
```
