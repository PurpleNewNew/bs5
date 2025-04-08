package socksproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strings"
)

type Socks5Client struct {
	proxy     *url.URL
	conf      *SOCKSConf
	tlsConfig *tls.Config
}

func (c *Socks5Client) Dial(ctx context.Context, network, address string) (remoteConn net.Conn, err error) {
	host, port, err := splitHostPort(address)
	if err != nil {
		return
	}
	request := &socks5Request{
		version: socks5version,
		socks5Addr: &socks5Addr{
			addrType: socks5AddressTypeFQDN,
			addr:     append([]byte{byte(len(host))}, host...),
			port:     port,
		},
	}
	if request.command, err = c.commandByNetwork(network); err != nil {
		return
	}

	if remoteConn, err = c.conf.Dial(ctx, network, c.proxy.Host); err != nil {
		return
	}
	if c.isTLS() {
		tlsConn := tls.Client(remoteConn, c.tlsConfig)
		if err = tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			return
		}
		remoteConn = tlsConn
	}
	if err = c.handshake(remoteConn); err != nil {
		return
	}
	if _, err = remoteConn.Write(request.ToPacket()); err != nil {
		return
	}
	switch request.command {
	case commandConnect:
		err = c.handleConnect(remoteConn)
	case commandUDPAssociate:
		err = c.handleUDPAssociate(remoteConn)
	}
	return
}

func (c *Socks5Client) handshake(conn net.Conn) (err error) {
	method := socks5AuthMethodNoRequired
	if c.proxy.User != nil && c.proxy.User.Username() != "" {
		method = socks5AuthMethodPassword
	}
	if c.isTLS() {
		method += 0x80
	}
	request := &socks5InitialRequest{
		version: socks5version,
		methods: []byte{method},
	}

	if _, err = conn.Write(request.ToPacket()); err != nil {
		return
	}

	reader := bufio.NewReader(conn)
	version, err := reader.ReadByte()
	if err != nil {
		return
	}
	if version != socks5version {
		return errVersionError
	}
	auth, err := reader.ReadByte()
	if err != nil {
		return
	}
	switch {
	case auth == 2 && method == socks5AuthMethodPassword:
		passed, err := c.passwordAuth(conn)
		if err != nil {
			return err
		}
		if !passed {
			err = errors.New("password authentication failed.")
		}
	case auth != 0 && method == socks5AuthMethodNoRequired:
		err = errors.New("socks method negotiation failed.")
	}
	return
}

func (c *Socks5Client) passwordAuth(conn net.Conn) (bool, error) {
	username := c.proxy.User.Username()
	password, _ := c.proxy.User.Password()
	request := []byte{1}
	request = append(request, byte(len(username)))
	request = append(request, []byte(username)...)
	request = append(request, byte(len(password)))
	request = append(request, []byte(password)...)
	if _, err := conn.Write(request); err != nil {
		return false, err
	}
	response := make([]byte, 2)
	if _, err := conn.Read(response); err != nil {
		return false, err
	}
	if response[0] != 0x01 {
		return false, errors.New("unexpected auth")
	}
	return response[1] == 0, nil
}

func (c *Socks5Client) commandByNetwork(network string) (command byte, err error) {
	switch strings.ToLower(network) {
	case "tcp", "tcp4", "tcp6":
		command = commandConnect
		return
	case "udp", "udp4", "udp6":
		command = commandUDPAssociate
		return
	default:
		err = errCommandNotSupported
		return
	}
}

func (c *Socks5Client) handleConnect(conn net.Conn) (err error) {
	reader := bufio.NewReader(conn)
	version, err := reader.ReadByte()
	if err != nil {
		return
	}
	if version != socks5version {
		return errVersionError
	}
	status, err := reader.ReadByte()
	if err != nil {
		return
	}
	if status != 0 {
		return errors.New("Can't complete SOCKS5 connection.")
	}
	// skip reserved
	if _, err = reader.Discard(1); err != nil {
		return
	}
	if _, err = readSocks5Addr(reader); err != nil {
		return
	}
	return
}

func (c *Socks5Client) handleUDPAssociate(conn net.Conn) (err error) {
	reader := bufio.NewReader(conn)
	// skip reserved
	if _, err = reader.Discard(2); err != nil {
		return
	}
	// skip fragment
	if _, err = reader.Discard(1); err != nil {
		return
	}
	if _, err = readSocks5Addr(reader); err != nil {
		return
	}
	return
}

func (c *Socks5Client) isTLS() bool {
	return strings.ToUpper(c.proxy.Scheme) == "SOCKS5+TLS"
}
