# ss-go

Go 编写的标准 Shadowsocks TCP 代理，支持 AEAD 和旧客户端的 AES-256-CFB。AEAD 协议实现复用 `github.com/shadowsocks/go-shadowsocks2 v0.1.5`。不再使用自定义用户 ID 或自定义握手。

## Linux 服务端

将 `dist/ssgo-linux-amd64/` 中的 `ssgo` 和 `config.json` 上传到 Linux x86_64 服务器的同一目录。修改密码后，在该目录执行：

```bash
chmod 700 ssgo
chmod 600 config.json
./ssgo
```

默认读取**当前工作目录**的 `config.json`；其他位置可使用 `./ssgo -config /path/to/config.json`。后台启动：

```bash
nohup ./ssgo > /dev/null 2>&1 &
```

运行日志自动写入**当前工作目录**的 `log/YYYY-MM-DD.log`，例如 `log/2026-09-30.log`。目录自动创建，按服务器系统本地时区分日；跨天后的第一条日志自动切换文件，重启当天继续追加，不覆盖历史日志。正常运行不再向终端输出日志；配置错误同样记录到日志文件。日志目录无法创建时启动失败并向终端报错，运行期间写文件失败时退回标准错误输出。

例如从 `/data/www/ss-go` 启动时，日志位于 `/data/www/ss-go/log/`：

```bash
tail -f "log/$(date +%F).log"
```

日志暂不自动清理，请按需要保留或删除历史文件。

配置示例（完整示例见 `config.example.json`）：

```json
{
  "mode": "server",
  "server": { "dial_timeout_seconds": 10 },
  "users": [
    { "listen": ":1818", "password": "replace-with-a-long-random-password", "method": "aes-256-cfb" },
    { "listen": ":1819", "password": "replace-with-another-long-random-password", "method": "aes-256-cfb" }
  ]
}
```

本项目采用每个用户一个监听端口，不需要用户名或 `id`。多个用户必须分别填写 `users[].listen`。单用户也允许省略该字段并使用 `server.listen`。服务器安全组和防火墙放行实际配置的 TCP 端口。配置修改后重启生效。

## 小飞机如何配置

支持旧版小飞机的 **aes-256-cfb + origin + plain** 组合；截图中具体客户端版本尚未直接实测。

| 客户端字段 | 配置 |
| --- | --- |
| 地址 | 你的服务器 IP 或域名 |
| 端口 | 对应用户的端口，例如 1818 |
| 加密方法 | 与服务端一致，当前配置为 aes-256-cfb |
| 密码 | 对应 users 条目里的 password |
| SSR 协议（如有） | origin（客户端需支持兼容标准 SS 的模式） |
| 混淆（如有） | plain / 无 |
| 协议参数、混淆参数 | 留空 |

当前根目录和部署目录配置使用 `aes-256-cfb`，端口为 `1818`。旧客户端保持 `origin` 和 `plain`；不支持 `http_simple`。已经使用小飞机时，本机无需再运行 ss-go 客户端。

支持 `aes-256-cfb`、`aes-128-gcm`、`aes-256-gcm`、`chacha20-ietf-poly1305`。CFB 按旧版 Shadowsocks 的 EVP_BytesToKey 派生密钥，每个连接、每个方向使用独立随机 IV；它没有完整性校验或重放保护，仅用于旧客户端兼容。有条件时应使用 AEAD；省略 method 仍默认 aes-256-gcm。不支持 SSR 扩展、Shadowsocks 2022、UDP、TUN、混淆或图形界面。

## 可选的命令行客户端

本机没有小飞机时，可以运行内置 SOCKS5 客户端。客户端只需自己的密码，不需要完整用户列表：

```json
{
  "mode": "client",
  "client": {
    "listen": "127.0.0.1:1080",
    "server": "your-server.example.com:1818",
    "password": "replace-with-a-long-random-password",
    "method": "aes-256-cfb"
  }
}
```

```bash
go run .
# 同时包含 server/client 配置时，可覆盖 mode：
go run . -mode client
curl --proxy socks5h://127.0.0.1:1080 https://example.com
```

SOCKS5 的成功响应表示本地请求已交给加密连接。标准 Shadowsocks 没有独立的登录确认或目标连接成功回执，因此 AEAD 错误密码或目标不可达会表现为后续连接关闭；CFB 没有密码认证，错误密码通常表现为地址解析失败、连接关闭或超时，不能仅凭 SOCKS5 成功响应判定密码正确。域名形式的请求在服务端解析；IP 形式直接转发。支持 TCP 半关闭，半关闭后等待剩余响应最多 30 秒。

## 开发和构建

入口为根目录 `main.go`：

```bash
go run .
go build -o ssgo .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/ssgo-linux-amd64/ssgo .
```

Linux 产物为无需 Go 运行环境的静态二进制，保持未压缩目录。ARM64 服务器需要将构建参数改成 `GOARCH=arm64`。
