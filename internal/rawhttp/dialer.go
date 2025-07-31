package rawhttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type ContextDialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Dialer can dial a remote HTTP server.
type Dialer interface {
	// Dial dials a remote http server returning a Conn.
	Dial(protocol, addr string, options *Options) (net.Conn, error)
	DialWithProxy(protocol, addr string, upstream ContextDialFunc, timeout time.Duration, options *Options) (net.Conn, error)
	// Dial dials a remote http server with timeout returning a Conn.
	DialTimeout(protocol, addr string, timeout time.Duration, options *Options) (net.Conn, error)
}

type dialer struct {
	sync.Mutex                       // protects following fields
	conns      map[string][]net.Conn // maps addr to a, possibly empty, slice of existing Conns
}

func (d *dialer) Dial(protocol, addr string, options *Options) (net.Conn, error) {
	return d.dialTimeout(protocol, addr, 0, options)
}

func (d *dialer) DialTimeout(protocol, addr string, timeout time.Duration, options *Options) (net.Conn, error) {
	return d.dialTimeout(protocol, addr, timeout, options)
}

func (d *dialer) dialTimeout(protocol, addr string, timeout time.Duration, options *Options) (net.Conn, error) {
	d.Lock()
	if d.conns == nil {
		d.conns = make(map[string][]net.Conn)
	}
	if c, ok := d.conns[addr]; ok {
		if len(c) > 0 {
			conn := c[0]
			c[0] = c[len(c)-1]
			d.Unlock()
			return conn, nil
		}
	}
	d.Unlock()
	return clientDial(protocol, addr, timeout, options)
}

func (d *dialer) DialWithProxy(protocol, addr string, upstream ContextDialFunc, timeout time.Duration, options *Options) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := upstream(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("proxy error: %w", err)
	}
	if protocol == "https" {
		if conn, err = TlsHandshake(conn, addr, options); err != nil {
			if conn != nil {
				_ = conn.Close()
			}
			return nil, fmt.Errorf("tls handshake error: %w", err)
		}
	}
	return conn, nil
}

func clientDial(protocol, addr string, timeout time.Duration, options *Options) (net.Conn, error) {
	// http
	if protocol == "http" {
		if timeout > 0 {
			return net.DialTimeout("tcp", addr, timeout)
		}
		return net.Dial("tcp", addr)
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	// https
	if options.TLSHandshake != nil {
		return options.TLSHandshake(conn, addr, options)
	} else {
		return TlsHandshake(conn, addr, options)
	}
}

// TlsHandshake tls handshake on a plain connection
func TlsHandshake(conn net.Conn, addr string, options *Options) (net.Conn, error) {
	hostname := options.SNI
	if options.SNI == "" {
		colonPos := strings.LastIndex(addr, ":")
		if colonPos == -1 {
			colonPos = len(addr)
		}
		hostname = addr[:colonPos]
	}

	tlsConn := tls.Client(conn, &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         hostname,
	})
	return tlsConn, tlsConn.Handshake()
}
