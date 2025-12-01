package main

import (
	"flag"
	"log"

	"github.com/armon/go-socks5"
	"golang.org/x/net/proxy"
)

func main() {
	// --- Configuration Flags ---
	listenAddr := flag.String("listen", "127.0.0.1:1088", "Local SOCKS5 proxy listen address")
	localUser := flag.String("local-user", "", "Username for local SOCKS5 service (optional)")
	localPass := flag.String("local-pass", "", "Password for local SOCKS5 service (optional)")
	proxyAddr := flag.String("proxy", "127.0.0.1:1080", "Upstream remote SOCKS5 proxy address")
	upstreamUser := flag.String("user", "", "Upstream SOCKS5 username (optional)")
	upstreamPass := flag.String("pass", "", "Upstream SOCKS5 password (optional)")
	geoipDbPath := flag.String("geoip", "./GeoLite2-Country.mmdb", "Path to GeoLite2-Country.mmdb database")
	bypassCountries := flag.String("bypass-countries", "CN", "Comma-separated list of country codes to bypass upstream proxy")

	flag.Parse()

	// 1. Create the SmartResolver, which contains all routing logic.
	smartResolver, err := NewSmartResolver(*geoipDbPath, *bypassCountries)
	if err != nil {
		log.Fatalf("Failed to create smart resolver: %v", err)
	}
	defer smartResolver.Close()

	// 2. Create the upstream dialer for the SmartDialer.
	var upstreamAuth *proxy.Auth
	if *upstreamUser != "" {
		upstreamAuth = &proxy.Auth{User: *upstreamUser, Password: *upstreamPass}
	}
	upstreamSocksDialer, err := proxy.SOCKS5("tcp", *proxyAddr, upstreamAuth, proxy.Direct)
	if err != nil {
		log.Fatalf("Failed to create dialer for upstream SOCKS5 proxy: %v", err)
	}

	// 3. Create the SmartDialer, which is now a simple wrapper.
	smartDialer := &SmartDialer{
		socksDialer:  upstreamSocksDialer,
		systemDialer: proxy.Direct,
	}

	// 4. Configure the SOCKS5 server.
	conf := &socks5.Config{
		// Use our custom resolver to make routing decisions.
		Resolver: smartResolver,
		// Use our custom dialer to execute the decisions.
		Dial: smartDialer.Dial,
	}

	if *localUser != "" && *localPass != "" {
		creds := socks5.StaticCredentials{*localUser: *localPass}
		cator := socks5.UserPassAuthenticator{Credentials: creds}
		conf.AuthMethods = []socks5.Authenticator{cator}
		log.Printf("Username/password authentication enabled for local SOCKS5 service.")
	}

	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("Failed to create SOCKS5 server: %v", err)
	}

	// 5. Start the server.
	log.Printf("Smart SOCKS5 proxy server started, listening on: %s", *listenAddr)
	log.Printf("All traffic not destined for [%s] will be forwarded through upstream proxy %s", *bypassCountries, *proxyAddr)
	if err := server.ListenAndServe("tcp", *listenAddr); err != nil {
		log.Fatalf("Failed to start SOCKS5 server: %v", err)
	}
}