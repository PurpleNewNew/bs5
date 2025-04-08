package socksproxy

import (
	"context"
	"io"
	"net"
)

type socks4Conn struct {
	localConn net.Conn
	conf      *SOCKSConf
}

func (c *socks4Conn) Serve() (err error) {
	request, err := readSocks4Request(c.localConn)
	if err != nil {
		c.sendReply(request, socks4StatusRejected)
		return err
	}
	switch request.command {
	case commandConnect:
		c.sendReply(request, socks4StatusGranted)
		err = c.handleConnect(request.Address())
	default:
		err = errCommandNotSupported
	}
	if err != nil {
		c.sendReply(request, socks4StatusRejected)
	}
	return
}

func (c *socks4Conn) handleConnect(host string) (err error) {
	remoteConn, err := c.conf.Dial(context.Background(), "tcp", host)
	if err != nil {
		return err
	}
	go io.Copy(c.localConn, remoteConn)
	go io.Copy(remoteConn, c.localConn)
	return
}

func (c *socks4Conn) sendReply(request *socks4Request, status byte) {
	response := &socks4Response{
		status: status,
		port:   make([]byte, 2),
		ip:     make([]byte, 4),
	}
	if request.IsSOCKS4A() {
		response.port = request.port
		response.ip = request.ip
	}
	c.localConn.Write(response.ToPacket())
}
