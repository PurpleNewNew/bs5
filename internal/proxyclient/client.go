package proxyclient

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
)

type Dial func(ctx context.Context, network, address string) (net.Conn, error)

type DialFactory func(*url.URL, Dial) (Dial, error)

var DefaultDial = (&net.Dialer{}).DialContext
var schemes = map[string]DialFactory{}

func init() {
	RegisterScheme("DIRECT", newDirectProxyClient)
	RegisterScheme("REJECT", newRejectProxyClient)
	RegisterScheme("BLACKHOLE", newBlackholeProxyClient)
	RegisterScheme("SOCKS", newSocksProxyClient)
	RegisterScheme("SOCKS4", newSocksProxyClient)
	RegisterScheme("SOCKS4A", newSocksProxyClient)
	RegisterScheme("SOCKS5", newSocksProxyClient)
	RegisterScheme("SOCKS5+TLS", newSocksProxyClient)
	RegisterScheme("HTTP", newHTTPProxyClient)
	RegisterScheme("HTTPS", newHTTPProxyClient)
}

func NewClient(proxy *url.URL) (Dial, error) {
	return NewClientWithDial(proxy, DefaultDial)
}

func NewClientChain(proxies []*url.URL) (Dial, error) {
	return NewClientChainWithDial(proxies, DefaultDial)
}

func NewClientWithDial(proxy *url.URL, upstreamDial Dial) (_ Dial, err error) {
	if proxy == nil {
		err = errors.New("proxy url is nil")
		return
	}
	if upstreamDial == nil {
		err = errors.New("upstream dial is nil")
		return
	}
	proxy = normalizeLink(*proxy)
	var scheme string
	ss := strings.Split(proxy.Scheme, "+")
	scheme = ss[0]
	if _, ok := schemes[scheme]; !ok {
		err = errors.New("unsupported proxy client.")
		return
	} else {
		return schemes[scheme](proxy, upstreamDial)
	}
}

func NewClientChainWithDial(proxies []*url.URL, upstreamDial Dial) (dial Dial, err error) {
	dial = upstreamDial
	for _, proxyURL := range proxies {
		dial, err = NewClientWithDial(proxyURL, dial)
		if err != nil {
			return
		}
	}
	return
}

func RegisterScheme(schemeName string, factory DialFactory) {
	schemes[strings.ToUpper(schemeName)] = factory
}

func SupportedSchemes() []string {
	schemeNames := make([]string, 0, len(schemes))
	for schemeName := range schemes {
		schemeNames = append(schemeNames, schemeName)
	}
	return schemeNames
}

func (dial Dial) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dial(ctx, network, address)
}

func (dial Dial) TCPOnly(ctx context.Context, network, address string) (net.Conn, error) {
	switch strings.ToUpper(network) {
	case "TCP", "TCP4", "TCP6":
		return dial(ctx, network, address)
	default:
		return nil, errors.New("unsupported network type.")
	}
}

func (dial Dial) Dial(network, address string) (net.Conn, error) {
	return dial(context.Background(), network, address)
}
