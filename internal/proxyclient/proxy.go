package proxyclient

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"time"

	httpProxy "github.com/PurpleNewNew/bs5/internal/proxyclient/http"
	socksProxy "github.com/PurpleNewNew/bs5/internal/proxyclient/socks"
)

func init() {

}

func newDirectProxyClient(proxy *url.URL, _ Dial) (dial Dial, err error) {
	dial = (&net.Dialer{}).DialContext
	if timeout, _ := time.ParseDuration(proxy.Query().Get("timeout")); timeout != 0 {
		dial = DialWithTimeout(timeout)
	}
	return
}

func newRejectProxyClient(proxy *url.URL, _ Dial) (dial Dial, err error) {
	dialErr := errors.New("reject dial")
	dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, dialErr
	}
	if try, _ := strconv.ParseInt(proxy.Query().Get("try-to-blackhole"), 10, 8); try > 0 {
		attempt := int64(0)
		dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			attempt++
			if attempt > try {
				return blackholeConn{}, nil
			}
			return nil, dialErr
		}
	}
	return
}

func newHTTPProxyClient(proxy *url.URL, upstreamDial Dial) (dial Dial, err error) {
	client := httpProxy.Client{
		Proxy:        *proxy,
		TLSConfig:    tlsConfigByProxyURL(proxy),
		UpstreamDial: upstreamDial,
	}
	dial = Dial(client.Dial).TCPOnly
	return
}

func newSocksProxyClient(proxy *url.URL, upstreamDial Dial) (dial Dial, err error) {
	conf := &socksProxy.SOCKSConf{
		TLSConfig: tlsConfigByProxyURL(proxy),
		Dial:      upstreamDial,
	}
	client, err := socksProxy.NewClient(proxy, conf)
	if err != nil {
		return
	}
	dial = Dial(client.Dial).TCPOnly
	return
}
