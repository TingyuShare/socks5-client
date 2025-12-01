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

// proxyDecision 用于缓存决策结果
type proxyDecision int

const (
	decisionUnknown proxyDecision = iota
	decisionUseSocksProxy
	decisionBypassProxy // 修改为更通用的“Bypass”
)

// SmartDialer 结构体实现了 proxy.Dialer 接口
type SmartDialer struct {
	socksDialer     proxy.Dialer
	systemDialer    proxy.Dialer
	geoipReader     *geoip2.Reader
	dnsResolver     *net.Resolver
	cache           map[string]proxyDecision
	cacheMutex      sync.RWMutex
	bypassCountries map[string]bool // 新增：用于快速查找需要直连的国家
}

// NewSmartDialer 创建并初始化一个 SmartDialer
// 新增 bypassCountriesStr 参数
func NewSmartDialer(socksDialer proxy.Dialer, geoipDbPath string, bypassCountriesStr string) (*SmartDialer, error) {
	if geoipDbPath == "" {
		return nil, fmt.Errorf("GeoIP数据库路径不能为空")
	}

	reader, err := geoip2.Open(geoipDbPath)
	if err != nil {
		return nil, fmt.Errorf("无法打开 GeoIP 数据库: %w", err)
	}

	// 解析需要直连的国家列表
	bypassMap := make(map[string]bool)
	if bypassCountriesStr != "" {
		countries := strings.Split(bypassCountriesStr, ",")
		for _, country := range countries {
			bypassMap[strings.ToUpper(strings.TrimSpace(country))] = true
		}
	}
	log.Printf("配置的直连国家: %v", bypassMap)


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
		bypassCountries: bypassMap, // 初始化 bypassCountries
	}, nil
}

// Dial 是 SmartDialer 的核心方法，它决定使用哪个拨号器
func (s *SmartDialer) Dial(network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	// 1. 检查缓存
	s.cacheMutex.RLock()
	decision, found := s.cache[host]
	s.cacheMutex.RUnlock()

	if found {
		log.Printf("[缓存] 域名 %s 使用 %s", host, decisionToString(decision))
		return s.dialWithDecision(decision, network, addr)
	}

	// 2. 如果缓存未命中，则进行DNS和GeoIP查询
	log.Printf("[查询] 域名 %s, 使用 8.8.8.8 DNS...", host)
	ips, err := s.dnsResolver.LookupHost(context.Background(), host)
	if err != nil || len(ips) == 0 {
		log.Printf("[警告] DNS 解析失败 for %s: %v. 默认直连.", host, err)
		s.setCache(host, decisionBypassProxy)
		return s.systemDialer.Dial(network, addr)
	}

	ip := net.ParseIP(ips[0])
	log.Printf("[解析] 域名 %s -> IP %s", host, ip)

	record, err := s.geoipReader.Country(ip)
	if err != nil {
		log.Printf("[警告] GeoIP 查询失败 for %s: %v. 默认直连.", ip, err)
		s.setCache(host, decisionBypassProxy)
		return s.systemDialer.Dial(network, addr)
	}

	// 3. 根据GeoIP结果和直连列表做出决策
	// 检查解析出的国家代码是否在直连列表中
	if _, isBypass := s.bypassCountries[record.Country.IsoCode]; isBypass {
		log.Printf("[决策] IP %s (%s) 在直连国家列表中, 域名 %s 将直接连接", ip, record.Country.IsoCode, host)
		decision = decisionBypassProxy
	} else {
		log.Printf("[决策] IP %s (%s) 不在直连国家列表中, 域名 %s 将使用 SOCKS5 代理", ip, record.Country.IsoCode, host)
		decision = decisionUseSocksProxy
	}

	s.setCache(host, decision)
	return s.dialWithDecision(decision, network, addr)
}

// Close 关闭 GeoIP reader
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
	// 默认直连
	return s.systemDialer.Dial(network, addr)
}

func decisionToString(d proxyDecision) string {
	if d == decisionUseSocksProxy {
		return "SOCKS5 代理"
	}
	return "直接连接"
}
