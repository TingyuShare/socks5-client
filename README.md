# Smart SOCKS5 Proxy

这是一个用 Go 语言编写的智能 SOCKS5 代理服务器。它作为一个中间层，可以根据请求目的地的 IP 地址归属地，智能地决定是将流量直接发送到目标，还是通过一个上游的 SOCKS5 代理进行转发。

## 主要功能

- **本地SOCKS5服务**: 在本地启动一个完整的SOCKS5代理服务器。
- **智能流量分流**: 基于 [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) 数据库，自动识别目标 IP 的国家。
- **可配置的转发规则**: 您可以指定一个或多个国家代码，目的地为这些国家的流量将**直接连接**，不通过上游代理。
- **SOCKS5代理链**: 所有未被直连规则匹配的流量，都将被转发到您指定的上游 SOCKS5 代理。
- **双重认证支持**:
    - 可为本地SOCKS5服务设置用户名/密码。
    - 支持通过需要用户名/密码认证的上游SOCKS5代理进行转发。

## 环境要求

1.  **Go**: 建议版本 1.18 或更高。
2.  **GeoLite2 数据库**: 需要从 [MaxMind 官网](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) 下载 `GeoLite2-Country.mmdb` 文件。您需要注册一个免费账户才能下载。

## 如何使用

### 1. 准备

首先，将本项目克隆到本地:
```bash
git clone https://github.com/TingyuShare/socks5-client.git
cd socks5-client
```

然后，将您从 MaxMind 下载的 `GeoLite2-Country.mmdb` 文件放置在项目根目录下。

### 2. 启动服务

在项目根目录下运行 `go run .` 并附上您需要的参数。

**简单示例 (使用默认配置):**
```bash
# 启动本地SOCKS5服务，监听 127.0.0.1:1088
# 上游代理为 127.0.0.1:1080
# 目的地为中国(CN)的流量将直连
go run .
```

**复杂示例 (带认证和自定义规则):**
```bash
go run . -listen "127.0.0.1:9999" \
         -local-user "myuser" -local-pass "mypass" \
         -proxy "remote-server.com:1080" \
         -user "remote-user" -pass "remote-pass" \
         -bypass-countries "CN,RU,JP"
```

### 3. 配置客户端

将您的操作系统或应用程序（如浏览器）的代理设置为 **SOCKS5** 代理，并指向您启动的服务地址。

-   **服务器/IP**: `127.0.0.1`
-   **端口**: `1088` (或您通过 `-listen` 参数指定的端口)
-   **用户名/密码**: 您通过 `-local-user` 和 `-local-pass` 设置的凭据 (如果设置了)

## 命令行参数

| 参数 | 默认值 | 描述 |
| :--- | :--- | :--- |
| `-listen` | `127.0.0.1:1088` | 本地SOCKS5服务的监听地址和端口。 |
| `-local-user` | (空) | 本地SOCKS5服务的用户名。 |
| `-local-pass` | (空) | 本地SOCKS5服务的密码。 |
| `-proxy` | `127.0.0.1:1080` | 上游SOCKS5代理的地址和端口。 |
| `-user` | (空) | 上游SOCKS5代理的用户名。 |
| `-pass` | (空) | 上游SOCKS5代理的密码。 |
| `-geoip` | `./GeoLite2-Country.mmdb` | `GeoLite2-Country.mmdb` 数据库文件的路径。 |
| `-bypass-countries` | `CN` | 不需要走上游代理的国家代码列表，以逗号分隔。 |

```