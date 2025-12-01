package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/oschwald/geoip2-golang"
	"golang.org/x/net/proxy"
)

// proxyDecision is used to cache the routing decision.
type proxyDecision int

const (
	decisionUnknown proxyDecision = iota
	decisionUseSocksProxy
	decisionBypassProxy
)

// SmartDialer implements the proxy.Dialer interface.
type SmartDialer struct {
	socksDialer     proxy.Dialer
	systemDialer    proxy.Dialer
	geoipReader     *geoip2.Reader
	dnsResolver     *net.Resolver
	cache           map[string]proxyDecision
	cacheMutex      sync.RWMutex
	bypassCountries map[string]bool // For quick lookup of countries to bypass.
}

// NewSmartDialer creates and initializes a SmartDialer.
func NewSmartDialer(socksDialer proxy.Dialer, geoipDbPath string, bypassCountriesStr string) (*SmartDialer, error) {
	if geoipDbPath == "" {
		return nil, fmt.Errorf("GeoIP database path cannot be empty")
	}

	reader, err := geoip2.Open(geoipDbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database: %w", err)
	}

	// Parse the list of countries to bypass.
	bypassMap := make(map[string]bool)
	if bypassCountriesStr != "" {
		countries := strings.Split(bypassCountriesStr, ",")
		for _, country := range countries {
			bypassMap[strings.ToUpper(strings.TrimSpace(country))] = true
		}
	}
	log.Printf("Configured bypass countries: %v", bypassMap)

	return &SmartDialer{
		socksDialer:  socksDialer,
		systemDialer: proxy.Direct,
		geoipReader:  reader,
		dnsResolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", "8.8.8.8:53")
			},
		},
		cache:           make(map[string]proxyDecision),
		bypassCountries: bypassMap,
	}, nil
}

// Dial is the core method of SmartDialer that decides which dialer to use.
func (s *SmartDialer) Dial(network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	// 1. Check cache
	s.cacheMutex.RLock()
	decision, found := s.cache[host]
	s.cacheMutex.RUnlock()

	if found {
		log.Printf("[CACHE] Host %s will use %s", host, decisionToString(decision))
		return s.dialWithDecision(decision, network, addr)
	}

	// 2. If cache miss, perform DNS and GeoIP lookup
	log.Printf("[QUERY] Host %s, using 8.8.8.8 DNS...", host)
	ips, err := s.dnsResolver.LookupHost(context.Background(), host)
	if err != nil || len(ips) == 0 {
		log.Printf("[WARN] DNS lookup failed for %s: %v. Defaulting to direct connection.", host, err)
		s.setCache(host, decisionBypassProxy)
		return s.systemDialer.Dial(network, addr)
	}

	ip := net.ParseIP(ips[0])
	log.Printf("[RESOLVED] Host %s -> IP %s", host, ip)

	record, err := s.geoipReader.Country(ip)
	if err != nil {
		log.Printf("[WARN] GeoIP lookup failed for %s: %v. Defaulting to direct connection.", ip, err)
		s.setCache(host, decisionBypassProxy)
		return s.systemDialer.Dial(network, addr)
	}

	// 3. Make decision based on GeoIP result and bypass list
	if _, isBypass := s.bypassCountries[record.Country.IsoCode]; isBypass {
		log.Printf("[ROUTE] IP %s (%s) is in bypass list, host %s will connect directly", ip, record.Country.IsoCode, host)
		decision = decisionBypassProxy
	} else {
		log.Printf("[ROUTE] IP %s (%s) is not in bypass list, host %s will use SOCKS5 proxy", ip, record.Country.IsoCode, host)
		decision = decisionUseSocksProxy
	}

	s.setCache(host, decision)
	return s.dialWithDecision(decision, network, addr)
}

// Close closes the GeoIP reader.
func (s *SmartDialer) Close() {
	s.geoipReader.Close()
}

func (s *SmartDialer) setCache(host string, decision proxyDecision) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache[host] = decision
}

func (s *SmartDialer) dialWithDecision(decision proxyDecision, network, addr string) (net.Conn, error) {
	if decision == decisionUseSocksProxy {
		return s.socksDialer.Dial(network, addr)
	}
	// Default to direct connection
	return s.systemDialer.Dial(network, addr)
}

func decisionToString(d proxyDecision) string {
	if d == decisionUseSocksProxy {
		return "SOCKS5 Proxy"
	}
	return "Direct Connection"
}