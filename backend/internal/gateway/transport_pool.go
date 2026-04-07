package gateway

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// TransportPool 按账户+代理隔离的 HTTP Transport 连接池
// 确保不同账户的连接互不干扰，同一账户的连接可以复用
type TransportPool struct {
	mu         sync.RWMutex
	transports map[string]*http.Transport // key = poolKey(accountID, proxyURL)
}

// NewTransportPool 创建连接池
func NewTransportPool() *TransportPool {
	return &TransportPool{
		transports: make(map[string]*http.Transport),
	}
}

// poolKey 生成连接池 key：按账户ID + 代理URL 隔离
// 相同账户使用相同代理时复用连接，不同代理则隔离
func poolKey(accountID int64, proxyURL string) string {
	if proxyURL == "" {
		return "direct:" + itoa(accountID)
	}
	return "proxy:" + proxyURL + ":" + itoa(accountID)
}

// itoa 简单的 int64 转字符串，避免 import strconv
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// GetTransport 获取或创建指定账户的 Transport
func (p *TransportPool) GetTransport(accountID int64, proxyURL string) *http.Transport {
	key := poolKey(accountID, proxyURL)

	// 快路径：读锁检查
	p.mu.RLock()
	t, ok := p.transports[key]
	p.mu.RUnlock()
	if ok {
		return t
	}

	// 慢路径：写锁创建
	p.mu.Lock()
	defer p.mu.Unlock()

	// 双重检查
	if t, ok = p.transports[key]; ok {
		return t
	}

	t = &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	if proxyURL != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			t.Proxy = http.ProxyURL(parsed)
		}
	}

	p.transports[key] = t
	return t
}

// CloseIdle 关闭所有 Transport 的空闲连接
func (p *TransportPool) CloseIdle() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, t := range p.transports {
		t.CloseIdleConnections()
	}
}

// RemoveAccount 移除指定账户的 Transport（账户被禁用时清理）
func (p *TransportPool) RemoveAccount(accountID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	prefix1 := "direct:" + itoa(accountID)
	prefix2 := ":" + itoa(accountID)

	for key, t := range p.transports {
		if key == prefix1 || len(key) > 0 && key[len(key)-len(prefix2):] == prefix2 {
			t.CloseIdleConnections()
			delete(p.transports, key)
		}
	}
}
