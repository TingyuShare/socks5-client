package main

import (
	"flag"
	"log"

	"github.com/armon/go-socks5"
	"golang.org/x/net/proxy"
)

func main() {
	// --- 本地 SOCKS5 服务器配置 ---
	listenAddr := flag.String("listen", "127.0.0.1:1088", "本地SOCKS5代理监听地址")
	localUser := flag.String("local-user", "", "本地SOCKS5服务的用户名 (可选)")
	localPass := flag.String("local-pass", "", "本地SOCKS5服务的密码 (可选)")

	// --- 上游 SOCKS5 代理配置 ---
	proxyAddr := flag.String("proxy", "127.0.0.1:1080", "上游远端SOCKS5代理地址")
	upstreamUser := flag.String("user", "", "上游SOCKS5 用户名 (可选)")
	upstreamPass := flag.String("pass", "", "上游SOCKS5 密码 (可选)")

	// --- 智能转发规则配置 ---
	geoipDbPath := flag.String("geoip", "./GeoLite2-Country.mmdb", "GeoLite2-Country.mmdb 数据库路径")
	bypassCountries := flag.String("bypass-countries", "CN", "不需要使用上游代理的国家代码，以逗号分隔")

	flag.Parse()

	// 1. 创建 SmartDialer (逻辑不变)
	var upstreamAuth *proxy.Auth
	if *upstreamUser != "" {
		upstreamAuth = &proxy.Auth{User: *upstreamUser, Password: *upstreamPass}
	}
	upstreamSocksDialer, err := proxy.SOCKS5("tcp", *proxyAddr, upstreamAuth, proxy.Direct)
	if err != nil {
		log.Fatalf("无法创建指向上游SOCKS5代理的拨号器: %v", err)
	}

	smartDialer, err := NewSmartDialer(upstreamSocksDialer, *geoipDbPath, *bypassCountries)
	if err != nil {
		log.Fatalf("无法创建智能拨号器: %v", err)
	}
	defer smartDialer.Close()

	// 2. 配置SOCKS5服务器，并加入本地认证
	conf := &socks5.Config{
		Dial: smartDialer.Dial,
	}

	// 如果设置了本地用户名，则添加认证方法
	if *localUser != "" && *localPass != "" {
		creds := socks5.StaticCredentials{
			*localUser: *localPass,
		}
		cator := socks5.UserPassAuthenticator{Credentials: creds}
		conf.AuthMethods = []socks5.Authenticator{cator}
		log.Printf("本地SOCKS5服务已启用用户名/密码认证。")
	}


	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("创建SOCKS5服务器失败: %v", err)
	}

	// 3. 启动本地SOCKS5服务器
	log.Printf("智能SOCKS5代理服务器已启动，监听地址: %s", *listenAddr)
	if err := server.ListenAndServe("tcp", *listenAddr); err != nil {
		log.Fatalf("启动SOCKS5服务器失败: %v", err)
	}
}