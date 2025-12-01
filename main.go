package main

import (
	"flag"
	"log"

	"github.com/armon/go-socks5"
	"golang.org/x/net/proxy"
)

func main() {
	// --- Local SOCKS5 Server Configuration ---
	listenAddr := flag.String("listen", "127.0.0.1:1088", "Local SOCKS5 proxy listen address")
	localUser := flag.String("local-user", "", "Username for local SOCKS5 service (optional)")
	localPass := flag.String("local-pass", "", "Password for local SOCKS5 service (optional)")

	// --- Upstream SOCKS5 Proxy Configuration ---
	proxyAddr := flag.String("proxy", "127.0.0.1:1080", "Upstream remote SOCKS5 proxy address")
	upstreamUser := flag.String("user", "", "Upstream SOCKS5 username (optional)")
	upstreamPass := flag.String("pass", "", "Upstream SOCKS5 password (optional)")

	// --- Smart Routing Configuration ---
	geoipDbPath := flag.String("geoip", "./GeoLite2-Country.mmdb", "Path to GeoLite2-Country.mmdb database")
	bypassCountries := flag.String("bypass-countries", "CN", "Comma-separated list of country codes to bypass upstream proxy")

	flag.Parse()

	// 1. Create the SmartDialer, the core of our smart routing.
	var upstreamAuth *proxy.Auth
	if *upstreamUser != "" {
		upstreamAuth = &proxy.Auth{User: *upstreamUser, Password: *upstreamPass}
	}
	upstreamSocksDialer, err := proxy.SOCKS5("tcp", *proxyAddr, upstreamAuth, proxy.Direct)
	if err != nil {
		log.Fatalf("Failed to create dialer for upstream SOCKS5 proxy: %v", err)
	}

	smartDialer, err := NewSmartDialer(upstreamSocksDialer, *geoipDbPath, *bypassCountries)
	if err != nil {
		log.Fatalf("Failed to create smart dialer: %v", err)
	}
	defer smartDialer.Close()

	// 2. Configure the new SOCKS5 server with local authentication.
	conf := &socks5.Config{
		// Use our custom SmartDialer for all outgoing connections.
		Dial: smartDialer.Dial,
	}

	if *localUser != "" && *localPass != "" {
		creds := socks5.StaticCredentials{
			*localUser: *localPass,
		}
		cator := socks5.UserPassAuthenticator{Credentials: creds}
		conf.AuthMethods = []socks5.Authenticator{cator}
		log.Printf("Username/password authentication enabled for local SOCKS5 service.")
	}

	server, err := socks5.New(conf)
	if err != nil {
		log.Fatalf("Failed to create SOCKS5 server: %v", err)
	}

	// 3. Start the local SOCKS5 server.
	log.Printf("Smart SOCKS5 proxy server started, listening on: %s", *listenAddr)
	log.Printf("All traffic not destined for [%s] will be forwarded through upstream proxy %s", *bypassCountries, *proxyAddr)
	if err := server.ListenAndServe("tcp", *listenAddr); err != nil {
		log.Fatalf("Failed to start SOCKS5 server: %v", err)
	}
}
