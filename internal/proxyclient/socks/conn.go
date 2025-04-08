package socksproxy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
)

type SOCKSConf struct {
	Auth        func(username, password string) bool
	Dial        func(ctx context.Context, network, address string) (net.Conn, error)
	HandleError func(error)
	TLSConfig   *tls.Config
}

func Serve(listener net.Listener, conf *SOCKSConf) {
	if conf.HandleError == nil {
		conf.HandleError = func(_ error) {}
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			conf.HandleError(err)
			continue
		}
		go handleConn(conn, conf)
	}
}

func IsSOCKS(r io.Reader) bool {
	header := make([]byte, 1)
	if _, err := r.Read(header); err != nil {
		return false
	}
	return header[0] == 4 || header[0] == 5
}

func handleConn(conn net.Conn, conf *SOCKSConf) {
	var err error
	buffer := make([]byte, 1)
	if _, err = conn.Read(buffer); err != nil {
		conf.HandleError(err)
		return
	}
	switch buffer[0] {
	case socks4version:
		if conf.Auth != nil || conf.TLSConfig != nil {
			return
		}
		socksConn := &socks4Conn{conn, conf}
		err = socksConn.Serve()
	case socks5version:
		if conf.TLSConfig != nil {
			conn = tls.Server(conn, conf.TLSConfig)
		}
		socksConn := &socks5Conn{conn, conf}
		err = socksConn.Serve()
	}
	if err != nil {
		conf.HandleError(err)
		return
	}
}
