# Smart SOCKS5 Proxy

This is a smart SOCKS5 proxy server written in Go. It acts as an intermediary layer that intelligently decides whether to route traffic directly to the destination or forward it through an upstream SOCKS5 proxy, based on the GeoIP location of the destination IP address.

## Features

- **Local SOCKS5 Server**: Runs a full-featured SOCKS5 proxy server on your local machine.
- **Smart Traffic Routing**: Automatically identifies the destination country using the [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) database.
- **Configurable Routing Rules**: You can specify one or more country codes. Traffic destined for these countries will be connected **directly**, bypassing the upstream proxy.
- **SOCKS5 Proxy Chaining**: All traffic not matched by a bypass rule will be forwarded to your specified upstream SOCKS5 proxy.
- **Dual Authentication Support**:
    - Set a username/password for the local SOCKS5 service.
    - Forward traffic through an upstream SOCKS5 proxy that requires authentication.

## Prerequisites

1.  **Go**: Version 1.18 or higher is recommended.
2.  **GeoLite2 Database**: You need to download the `GeoLite2-Country.mmdb` file from the [official MaxMind website](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data). A free account is required for download.

## How to Use

### 1. Setup

First, clone the project to your local machine:
```bash
git clone https://github.com/TingyuShare/socks5-client.git
cd socks5-client
```

Next, place the `GeoLite2-Country.mmdb` file you downloaded from MaxMind into the project's root directory.

### 2. Run the Server

From the project's root directory, run `go run .` with your desired flags.

**Simple Example (Default Configuration):**
```bash
# Starts the local SOCKS5 service listening on 127.0.0.1:1088
# Upstream proxy is 127.0.0.1:1080
# Traffic destined for China (CN) will be connected directly
go run .
```

**Advanced Example (with authentication and custom rules):**
```bash
go run . -listen "127.0.0.1:9999" \
         -local-user "myuser" -local-pass "mypass" \
         -proxy "remote-server.com:1080" \
         -user "remote-user" -pass "remote-pass" \
         -bypass-countries "CN,RU,JP"
```

### 3. Configure Your Client

Set the proxy in your operating system or application (e.g., web browser) to use the **SOCKS5** proxy you just started.

-   **Server/IP**: `127.0.0.1`
-   **Port**: `1088` (or the port you specified with the `-listen` flag)
-   **Username/Password**: The credentials you set with `-local-user` and `-local-pass` (if any).

## Command-Line Flags

| Flag | Default | Description |
| :--- | :--- | :--- |
| `-listen` | `127.0.0.1:1088` | The listen address and port for the local SOCKS5 service. |
| `-local-user` | (empty) | Username for the local SOCKS5 service. |
| `-local-pass` | (empty) | Password for the local SOCKS5 service. |
| `-proxy` | `127.0.0.1:1080` | The address and port of the upstream SOCKS5 proxy. |
| `-user` | (empty) | Username for the upstream SOCKS5 proxy. |
| `-pass` | (empty) | Password for the upstream SOCKS5 proxy. |
| `-geoip` | `./GeoLite2-Country.mmdb` | Path to the `GeoLite2-Country.mmdb` database file. |
| `-bypass-countries` | `CN` | A comma-separated list of country codes to bypass the upstream proxy for. |

```