package socksproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type Socks4Client struct {
	proxy *url.URL
	conf  *SOCKSConf
}

func (c *Socks4Client) Dial(ctx context.Context, network, address string) (remoteConn net.Conn, err error) {
	host, port, err := splitHostPort(address)
	if err != nil {
		return
	}
	request := &socks4Request{
		command: commandConnect,
		port:    port,
		ip:      []byte{0, 0, 0, 1},
		userId:  []byte{},
		fqdn:    host,
	}
	if c.isSOCKS4() {
		request.ip, err = c.lookupIP(string(request.fqdn))
		if err != nil {
			return
		}
	}

	remoteConn, err = c.conf.Dial(ctx, "tcp", c.proxy.Host)
	if err != nil {
		return
	}
	if _, err = remoteConn.Write(request.ToPacket()); err != nil {
		return
	}

	switch request.command {
	case commandConnect:
		err = c.handleConnect(remoteConn)
	}
	return
}

func (c *Socks4Client) handleConnect(remoteConn net.Conn) (err error) {
	reader := bufio.NewReader(remoteConn)
	version, err := reader.ReadByte()
	if err != nil {
		return
	}
	if version != 0 {
		err = errVersionError
		return
	}
	code, err := reader.ReadByte()
	if err != nil {
		return
	}
	if code != socks4StatusGranted {
		switch code {
		case socks4StatusRejected:
			err = errors.New("Socks connection request rejected or failed.")
		case 92:
			err = errors.New("Socks connection request rejected becasue SOCKS server cannot connect to identd on the client.")
		case 93:
			err = errors.New("Socks connection request rejected because the client program and identd report different user-ids.")
		default:
			err = errors.New("Socks connection request failed, unknown error.")
		}
		return
	}
	_, err = reader.Discard(2) // dst port
	if err != nil {
		return
	}
	_, err = reader.Discard(4) // dst ip
	if err != nil {
		return
	}
	return
}

func (c *Socks4Client) isSOCKS4() bool {
	return strings.ToUpper(c.proxy.Scheme) == "SOCKS4"
}

func (c *Socks4Client) lookupIP(host string) (ip net.IP, err error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return
	}
	if len(ips) == 0 {
		err = fmt.Errorf("Cannot resolve host: %s.", host)
		return
	}
	ip = ips[0]
	if !isIPv4(ip) {
		err = errors.New("IPv6 is not supported by SOCKS4.")
		return
	}
	return
}
