package extend

import (
	"context"
	"github.com/PurpleNewNew/bs5/internal/proxyclient"
	"github.com/PurpleNewNew/bs5/internal/proxyclient/suo5"
	"net"
	"net/url"
)

func init() {
	proxyclient.RegisterScheme("SUO5", NewSuo5Client)
	proxyclient.RegisterScheme("SUO5S", NewSuo5Client)
}

// NewSuo5Client 创建一个新的 Suo5Client
func NewSuo5Client(proxy *url.URL, upstreamDial proxyclient.Dial) (dial proxyclient.Dial, err error) {
	conf, err := suo5.NewConfFromURL(proxy)
	if err != nil {
		return nil, err
	}
	if upstreamDial != nil {
		conf.ProxyClient = upstreamDial
	}
	c := &suo5.Suo5Client{
		Proxy: proxy,
		Conf:  conf,
	}

	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return c.Dial(network, address)
	}, nil
}
