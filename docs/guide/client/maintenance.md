# 客户端维护与更新

本页用于客户端连通后的状态检查、服务控制、更新和兼容处理。

## 状态检查

`npc status` 用于检查一组连接参数是否可用，不读取完整 `npc.conf` 运行态。

```bash
./npc status -server=127.0.0.1:8024 -vkey=YOUR_CLIENT_VKEY -type=tcp
```

使用单个 `-launch` 也可以：

```bash
./npc status -launch="npc://demo-vkey@127.0.0.1:8024?type=tcp"
```

限制：

- 不适合多 profile `-launch`
- 不适合直接检查配置文件里的全部隧道

## 服务控制

Linux / macOS：

```bash
sudo npc start
sudo npc stop
sudo npc restart
sudo npc uninstall
```

Windows：

```powershell
npc.exe start
npc.exe stop
npc.exe restart
npc.exe uninstall
```

如果安装服务时的连接参数需要变化，先卸载再重新安装。

## 日志

常用参数：

| 参数 | 说明 |
| --- | --- |
| `-log=stdout` | 输出到控制台 |
| `-log=file` | 输出到文件 |
| `-log=off` | 关闭日志 |
| `-log_level=info` | 设置日志级别 |
| `-log_path=<path>` | 指定日志路径 |

默认日志：

| 系统 | 路径 |
| --- | --- |
| Windows | `npc.exe` 同目录下的 `npc.log` |
| Linux / macOS | `/var/log/npc.log` |

## 更新

标准流程：

```bash
sudo npc stop
sudo npc update
sudo npc start
```

Windows：

```powershell
npc.exe stop
npc.exe update
npc.exe start
```

自动更新失败时，从 Releases 下载对应包，停止服务后替换二进制。

## 旧版兼容

新部署建议保持服务端 `secure_mode=true`。

确实要兼容旧客户端时：

1. 服务端确认是否需要临时关闭严格兼容限制
2. 旧客户端增加 `-proto_version=0`

```bash
./npc -server=ip:8024 -vkey=YOUR_CLIENT_VKEY -type=tcp -proto_version=0
```

如果仍无法连接，优先确认客户端和服务端版本是否属于兼容范围。
