<div align="center">

# bridge

**轻量级 TCP/UDP 端口转发器**

[![release](https://github.com/cuipeiyu/bridge/actions/workflows/release.yaml/badge.svg)](https://github.com/cuipeiyu/bridge/actions/workflows/release.yaml)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](#构建)

</div>

---

## 特性

- **TCP / UDP 双协议**转发，一条规则一个协议，规则数量不限
- **IPv4 / IPv6 双栈**：`[::]` 通配监听自动设置 `IPV6_V6ONLY`，可与 `0.0.0.0` 同端口共存、指向不同上游
- **UDP 会话管理**：按客户端地址保持会话，空闲自动老化回收，不泄漏 fd
- **干净退出**：`SIGINT`/`SIGTERM` 后立即回收所有会话，不阻塞
- **零依赖**：纯标准库，静态编译单文件，`CGO_ENABLED=0` 交叉编译开箱即用
- **systemd 集成**：附带 service 文件，崩溃自动拉起

## 工作原理

```
客户端 ──TCP/UDP──▶ bridge 监听 ListenAddr ──转发──▶ ToAddr 上游
                        │
                        └─ UDP：按客户端地址维持会话，空闲 TTL 1 分钟自动老化
```

## 快速开始

### 一键安装（Linux）

```bash
curl -fsSL https://raw.githubusercontent.com/cuipeiyu/bridge/main/install.sh | bash
```

### 手动安装

1. 从 [Releases](https://github.com/cuipeiyu/bridge/releases) 下载对应平台的 `bridge-linux-amd64.tar.gz`
2. 解压后将 `bridge`、`bridge.conf` 放入 `/usr/local/sbin/`，`bridge.service` 放入 `/usr/lib/systemd/system/`
3. 启动：

```bash
systemctl enable --now bridge
```

### 服务管理

```bash
systemctl start bridge      # 启动
systemctl restart bridge    # 重启（修改配置后）
systemctl status bridge     # 状态
journalctl -u bridge -f     # 实时日志
```

## 配置

配置文件 `bridge.conf` 位于进程工作目录（systemd 部署时为 `/usr/local/sbin/`）。每行一条规则：

```
ListenAddr  ToAddr  Proto
```

| 字段 | 说明 | 示例 |
|---|---|---|
| `ListenAddr` | 本机监听地址 | `0.0.0.0:80`、`[::]:443` |
| `ToAddr` | 上游地址（支持域名） | `127.0.0.1:8080`、`example.com:666` |
| `Proto` | 协议，仅 `tcp` / `udp`（大小写不敏感） | `tcp` |

示例：

```conf
# 每行一条：ListenAddr ToAddr Proto(tcp/udp)，# // 开头为注释
0.0.0.0:80 127.0.0.1:99 tcp
[::]:443 my.domain.com:666 tcp
0.0.0.0:53 8.8.8.8:53 udp
```

注意事项：

- **双栈共存**：`[::]` 监听会设置 `IPV6_V6ONLY=1`，不接收 IPv4 流量，因此可与 `0.0.0.0` 同端口并存、分别指向不同上游
- **不要写 v4-mapped 地址**（如 `[::ffff:1.2.3.4]`），设置 V6ONLY 后会 `bind EINVAL`，需要 IPv4 直接写 IPv4 地址
- 非法协议、字段不足的行会记录日志并跳过，不影响其他规则

## 构建

```bash
make release             # 全平台发布包（linux/darwin amd64+arm64、windows）
make help                # 查看全部目标
```

运行测试：

```bash
go test -race ./...
```

## 可选：定时重启

```bash
crontab -e
# 每 2 小时重启一次
* */2 * * * systemctl restart bridge
```

## License

[Apache-2.0](LICENSE)
