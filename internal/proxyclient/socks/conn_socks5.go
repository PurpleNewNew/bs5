package socksproxy

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"syscall"
)

type socks5Conn struct {
	localConn net.Conn
	conf      *SOCKSConf
}

func (c *socks5Conn) Serve() (err error) {
	err = c.handshake()
	if err == errAuthMethodNotSupported {
		c.localConn.Close()
		return
	}
	if err != nil {
		return
	}
	request, err := readSocks5Request(c.localConn)
	if err == errAddressTypeNotSupported {
		c.sendReply(request, socks5StatusAddressTypeNotSupported)
		return
	}
	if err != nil {
		return
	}
	if request.version != socks5version {
		return errVersionError
	}
	switch request.command {
	case commandConnect:
		err = c.handleConnect(request)
	case commandUDPAssociate:
		err = c.handleUDPAssociate(request)
	default:
		c.sendReply(request, socks5StatusCommandNotSupported)
		return errCommandNotSupported
	}
	return
}

func (c *socks5Conn) handleConnect(request *socks5Request) (err error) {
	c.sendReply(request, socks5StatusSucceeded)
	remoteConn, err := c.conf.Dial(context.Background(), "tcp", request.Address())
	if c.sendReplyWithError(request, err) {
		return
	}
	go io.Copy(c.localConn, remoteConn)
	go io.Copy(remoteConn, c.localConn)
	return
}

func (c *socks5Conn) handleUDPAssociate(request *socks5Request) (err error) {
	c.sendUDPReply(request)
	remoteConn, err := c.conf.Dial(context.Background(), "udp", request.Address())
	if c.sendReplyWithError(request, err) {
		return err
	}
	go io.Copy(c.localConn, remoteConn)
	go io.Copy(remoteConn, c.localConn)
	return
}

func (c *socks5Conn) handshake() (err error) {
	reader := bufio.NewReader(c.localConn)
	method, err := reader.ReadByte()
	if err != nil {
		return
	}
	methods := make([]byte, method)
	if _, err = reader.Read(methods); err != nil {
		return
	}
	if c.conf.Auth == nil {
		c.sendAuthReply(socks5AuthMethodNoRequired)
		return
	}
	if err = c.authBasedPassword(methods); err != nil {
		return
	}
	return
}

func (c *socks5Conn) authBasedPassword(methods []byte) (err error) {
	method := socks5AuthMethodPassword
	if c.isTLS() {
		method = socks5AuthMethodTLSPassword
	}
	if !bytes.Contains(methods, []byte{method}) {
		c.sendAuthReply(socks5AuthMethodNoAcceptable)
		return errAuthMethodNotSupported
	}
	c.sendAuthReply(method)

	reader := bufio.NewReader(c.localConn)
	version, err := reader.ReadByte()
	if version != 0x01 {
		return errAuthMethodNotSupported
	}
	usernameLength, err := reader.ReadByte()
	if err != nil {
		return
	}
	username := make([]byte, usernameLength)
	if _, err = reader.Read(username); err != nil {
		return
	}
	passwordLength, err := reader.ReadByte()
	if err != nil {
		return
	}
	password := make([]byte, passwordLength)
	if _, err = reader.Read(password); err != nil {
		return
	}
	if c.conf.Auth(string(username), string(password)) {
		c.localConn.Write([]byte{0x01, 0x01})
	} else {
		c.localConn.Write([]byte{0x01, 0x00})
	}
	return
}

func (c *socks5Conn) sendReply(request *socks5Request, status byte) {
	reply := []byte{socks5version, status, 0x00}
	hostName, hostPort, _ := splitHostPort(c.localConn.LocalAddr().String())
	ip := net.ParseIP(string(hostName))
	addrType := socks5AddressTypeIPv4
	if ip.To16() != nil {
		addrType = socks5AddressTypeIPv6
	}
	reply = append(reply, addrType)
	reply = append(reply, ip...)
	reply = append(reply, hostPort...)
	c.localConn.Write(reply)
	if status != socks5StatusSucceeded {
		c.localConn.Close()
	}
}

func (c *socks5Conn) sendReplyWithError(request *socks5Request, err error) bool {
	if err == nil {
		return false
	}
	switch err := err.(type) {
	case net.Error:
		if err.Timeout() {
			c.sendReply(request, socks5StatusHostUnreachable)
		}
	case *net.OpError:
		switch err.Op {
		case "dial":
			c.sendReply(request, socks5StatusHostUnreachable)
		case "read":
			c.sendReply(request, socks5StatusConnectionRefused)
		}
	case syscall.Errno:
		switch err {
		case syscall.ECONNREFUSED:
			c.sendReply(request, socks5StatusConnectionRefused)
		}
	default:
		c.sendReply(request, socks5StatusGeneral)
	}
	return true
}

func (c *socks5Conn) sendAuthReply(status byte) {
	c.localConn.Write([]byte{socks5version, status})
}

func (c *socks5Conn) sendUDPReply(request *socks5Request) {
	reply := []byte{0, 0}
	fragment := byte(0)
	reply = append(reply, fragment)
	reply = append(reply, request.addrType)
	reply = append(reply, request.addr...)
	reply = append(reply, request.port...)
	c.localConn.Write(reply)
}

func (c *socks5Conn) isTLS() bool {
	return c.conf.TLSConfig != nil
}
