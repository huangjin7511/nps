# 启动 NPS 服务端

本页只讲启动服务端并进入 Web 管理界面。还没安装时先看 [安装指南](/getting-started/install.md)。

## 前台验证

第一次建议先前台运行，方便看到报错和确认配置目录。

Linux / macOS 标准安装：

```bash
nps -conf_path=/etc/nps
```

发布包目录内运行：

```bash
./nps
```

Windows：

```powershell
nps.exe
```

自定义配置目录：

```bash
nps -conf_path=/app/nps
```

```powershell
nps.exe -conf_path=D:\nps
```

## 注册服务

前台验证正常后，再注册系统服务。

Linux / macOS：

```bash
sudo nps install
sudo nps start
```

Windows 建议使用管理员 PowerShell：

```powershell
nps.exe install
nps.exe start
```

如果要指定自定义配置目录，在 `install` 时传一次即可：

```bash
nps install -conf_path=/app/nps
```

```powershell
nps.exe install -conf_path=D:\nps
```

后续执行 `nps start` 或 `nps.exe start`，不需要重复传 `-conf_path`。

## 常用服务命令

| 操作 | Linux / macOS | Windows |
| --- | --- | --- |
| 启动 | `sudo nps start` | `nps.exe start` |
| 停止 | `sudo nps stop` | `nps.exe stop` |
| 重启 | `sudo nps restart` | `nps.exe restart` |
| 卸载服务 | `sudo nps uninstall` | `nps.exe uninstall` |

## 配置位置

| 场景 | 配置目录 | 配置文件 |
| --- | --- | --- |
| Linux / macOS 标准安装 | `/etc/nps` | `/etc/nps/conf/nps.conf` |
| Windows 管理员安装 | `C:\Program Files\nps` | `C:\Program Files\nps\conf\nps.conf` |
| Windows 无管理员脚本安装 | `%LOCALAPPDATA%\nps` | `%LOCALAPPDATA%\nps\conf\nps.conf` |
| 自定义目录 | `-conf_path` 指定目录 | 该目录下的 `conf/nps.conf` |

Linux / macOS 默认日志通常是 `/var/log/nps.log`。Windows 默认日志在当前运行的 `nps.exe` 所在目录。

## 登录 Web 管理界面

仓库示例配置默认访问：

```text
http://<server-ip>:8081
```

示例账号密码：

```text
admin / 123
```

账号密码来自 `nps.conf` 的 `web_username` 和 `web_password`。正式环境应修改配置文件后重启 NPS。
