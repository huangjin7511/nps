# Windows 安装脚本

本页介绍仓库根目录的 `install.ps1`。熟悉发布包选择时，也可以直接看 [发布包安装](/getting-started/install-release.md)。

## 适合场景

- 不确定该下载哪个架构。
- 需要兼容 Windows 7 / 8 / 8.1。
- 没有管理员权限，不确定安装目录。
- GitHub 下载不稳定。

## 默认行为

脚本默认非交互执行，不加 `-Menu` 不会弹出菜单。

| 项 | 默认值 |
| --- | --- |
| 安装模式 | `all` |
| 版本 | `latest` |
| 架构 | 自动检测 |
| 包类型 | Windows 7 / 8 / 8.1 使用 `old`，Windows 10 / 11 使用普通包 |
| 管理员目录 | `C:\Program Files\nps` |
| 非管理员目录 | `%LOCALAPPDATA%\nps` |

下载会先尝试 GitHub Release / API，失败后尝试 jsDelivr 镜像。

## 下载脚本

```powershell
Invoke-WebRequest -UseBasicParsing -OutFile .\install.ps1 https://fastly.jsdelivr.net/gh/djylb/nps@master/install.ps1
```

备用地址：

```powershell
Invoke-WebRequest -UseBasicParsing -OutFile .\install.ps1 https://cdn.jsdelivr.net/gh/djylb/nps@master/install.ps1
```

## 安装

安装服务端：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 nps
```

安装客户端：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 npc
```

同时安装：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

菜单模式：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Menu
```

## 常用参数

安装到自定义目录：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 nps latest D:\nps
```

指定架构：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 npc -Arch amd64
```

指定包类型：

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1 nps -PackageVariant modern
powershell -ExecutionPolicy Bypass -File .\install.ps1 nps -PackageVariant old
```

## 安装后

脚本只负责下载、解压和放置文件。注册服务仍使用二进制命令：

- 服务端看 [启动 NPS 服务端](/getting-started/start-server.md)
- 客户端看 [启动 NPC 客户端](/getting-started/start-client.md)
