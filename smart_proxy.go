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

type contextKey string
const decisionKey contextKey = "routing-decision"

// --- 1. SmartResolver: Decides the route and passes it via context ---

type SmartResolver struct {
	geoipReader     *geoip2.Reader
	dnsResolver     *net.Resolver // Custom DNS resolver (e.g., 8.8.8.8)
	cache           map[string]proxyDecision
	cacheMutex      sync.RWMutex
	bypassCountries map[string]bool
}

// NewSmartResolver creates a new resolver responsible for routing decisions.
func NewSmartResolver(geoipDbPath string, bypassCountriesStr string) (*SmartResolver, error) {
	if geoipDbPath == "" {
		return nil, fmt.Errorf("GeoIP database path cannot be empty")
	}
	reader, err := geoip2.Open(geoipDbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GeoIP database: %w", err)
	}

	bypassMap := make(map[string]bool)
	if bypassCountriesStr != "" {
		for _, country := range strings.Split(bypassCountriesStr, ",") {
			bypassMap[strings.ToUpper(strings.TrimSpace(country))] = true
		}
	}
	log.Printf("Configured bypass countries: %v", bypassMap)

	return &SmartResolver{
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

// Resolve performs the routing logic and returns a new context with the decision.
// It returns a nil IP to force the socks5 library to pass the original hostname to the Dialer.
func (s *SmartResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// 1. Check cache
	s.cacheMutex.RLock()
	decision, found := s.cache[name]
	s.cacheMutex.RUnlock()

	if !found {
		// 2. If cache miss, perform DNS and GeoIP lookup
		log.Printf("[QUERY] Host %s, using 8.8.8.8 DNS...", name)
		ips, err := s.dnsResolver.LookupHost(ctx, name)
		// If DNS lookup fails (e.g., "no such host"), we default to a direct connection.
		if err != nil || len(ips) == 0 {
			log.Printf("[WARN] DNS lookup failed for %s: %v. Defaulting to direct connection.", name, err)
			decision = decisionBypassProxy
		} else {
			ip := net.ParseIP(ips[0])
			log.Printf("[RESOLVED] Host %s -> IP %s", name, ip)
			record, err := s.geoipReader.Country(ip)
			if err != nil {
				log.Printf("[WARN] GeoIP lookup failed for %s: %v. Defaulting to direct connection.", ip, err)
				decision = decisionBypassProxy
			} else {
				// 3. Make decision based on GeoIP result and bypass list
				if _, isBypass := s.bypassCountries[record.Country.IsoCode]; isBypass {
					log.Printf("[ROUTE] IP %s (%s) is in bypass list, host %s will connect directly", ip, record.Country.IsoCode, name)
					decision = decisionBypassProxy
				} else {
					log.Printf("[ROUTE] IP %s (%s) is not in bypass list, host %s will use SOCKS5 proxy", ip, record.Country.IsoCode, name)
					decision = decisionUseSocksProxy
				}
			}
		}
		s.setCache(name, decision)
	}

	// 4. Store decision in the context and return
	newCtx := context.WithValue(ctx, decisionKey, decision)
	return newCtx, nil, nil
}

func (s *SmartResolver) setCache(host string, decision proxyDecision) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	s.cache[host] = decision
}

func (s *SmartResolver) Close() {
	s.geoipReader.Close()
}


// --- 2. SmartDialer: Reads the decision from context and dials ---

type SmartDialer struct {
	socksDialer  proxy.Dialer
	systemDialer proxy.Dialer
}

// Dial reads the routing decision from the context and uses the appropriate dialer.
func (s *SmartDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	decision, _ := ctx.Value(decisionKey).(proxyDecision)
	
	log.Printf("[DIAL] Host %s will use %s", addr, decisionToString(decision))

	type ContextDialer interface {
		DialContext(ctx context.Context, network, addr string) (net.Conn, error)
	}

	if decision == decisionUseSocksProxy {
		if d, ok := s.socksDialer.(ContextDialer); ok {
			return d.DialContext(ctx, network, addr)
		}
	}

	// Default to direct connection
	if d, ok := s.systemDialer.(ContextDialer); ok {
		return d.DialContext(ctx, network, addr)
	}
	
	return s.systemDialer.Dial(network, addr)
}

func decisionToString(d proxyDecision) string {
	if d == decisionUseSocksProxy {
		return "SOCKS5 Proxy"
	}
	return "Direct Connection"
}