# 重载、重启与更新

本页只讲服务生命周期：重载、重启、更新和手动覆盖。

## 重载还是重启

`nps reload` 只适合非 Windows 平台的少量运行态配置变更。它会重新加载配置并替换部分运行态状态，但不会重建所有监听器和协议入口。

| 操作 | 适合变更 |
| --- | --- |
| `reload` | 日志、时区、DNS/NTP、登录 ACL、访问策略、允许端口、部分 Web 管理和鉴权运行态配置 |
| `restart` | `run_mode`、Web 监听、HTTP/HTTPS 代理入口、Bridge 端口、P2P 端口、QUIC 参数、`pprof_*` |

`reload` 需要能定位运行中的 `nps.pid` 并发送信号。失败时直接重启。

Linux / macOS：

```bash
sudo nps reload
```

Windows 当前不支持 `reload`，请重启：

```powershell
nps.exe restart
```

## 停止与重启

Linux / macOS：

```bash
sudo nps stop
sudo nps restart
```

Windows：

```powershell
nps.exe stop
nps.exe restart
```

## 更新

标准流程是停止、更新、启动。

Linux / macOS：

```bash
sudo nps stop
sudo nps update
sudo nps start
```

Windows：

```powershell
nps.exe stop
nps.exe update
nps.exe start
```

旧入口仍兼容：

```bash
sudo nps-update update
```

```powershell
nps-update.exe update
```

如果自动更新失败，从 [GitHub Releases](https://github.com/djylb/nps/releases/latest) 下载对应发布包，并同时替换二进制文件和 `web` 目录。

## 手动覆盖

适用于自动更新失败、回滚或切换指定版本。

Linux / macOS：

```bash
sudo systemctl stop nps
whereis nps
sudo cp nps /usr/bin/nps
sudo chmod +x /usr/bin/nps
sudo systemctl start nps
```

Windows：

```powershell
Stop-Service nps
Copy-Item -Path "PATH_TO_NEW_NPS_EXE" -Destination "PATH_TO_OLD_NPS_EXE_DIR" -Force
Start-Service nps
```
