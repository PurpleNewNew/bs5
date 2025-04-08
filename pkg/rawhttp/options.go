package rawhttp

import (
	"net"
	"time"

	"github.com/OmagariHare/bs5/pkg/rawhttp/client"
)

// Options contains configuration options for rawhttp client
type Options struct {
	Timeout                time.Duration
	FollowRedirects        bool
	MaxRedirects           int
	AutomaticHostHeader    bool
	AutomaticContentLength bool
	CustomHeaders          client.Headers
	ForceReadAllBody       bool // ignores content length and reads all body
	CustomRawBytes         []byte
	Proxy                  ContextDialFunc
	ProxyDialTimeout       time.Duration
	SNI                    string
	TLSHandshake           func(conn net.Conn, addr string, options *Options) (net.Conn, error)
}

// DefaultOptions is the default configuration options for the client
var DefaultOptions = &Options{
	Timeout:                30 * time.Second,
	FollowRedirects:        true,
	MaxRedirects:           10,
	AutomaticHostHeader:    true,
	AutomaticContentLength: true,
}
