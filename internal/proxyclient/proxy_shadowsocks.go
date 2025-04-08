package proxyclient

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"

	ss "github.com/shadowsocks/go-shadowsocks2/core"
)

// buildSSAddr 构造 Shadowsocks 请求头
// 格式: ATYP + DST.ADDR + DST.PORT
// ATYP: 1字节, 0x01 = IPv4, 0x03 = 域名, 0x04 = IPv6
// DST.ADDR: 变长,根据 ATYP 决定
// DST.PORT: 2字节,网络字节序
func buildSSAddr(host string, port int) ([]byte, error) {
	if len(host) == 0 {
		return nil, errors.New("empty host")
	}

	var buf []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			buf = make([]byte, 1+net.IPv4len+2)
			buf[0] = 0x01 // IPv4
			copy(buf[1:], ip4)
		} else {
			buf = make([]byte, 1+net.IPv6len+2)
			buf[0] = 0x04 // IPv6
			copy(buf[1:], ip)
		}
	} else {
		if len(host) > 255 {
			return nil, errors.New("target host name too long")
		}
		buf = make([]byte, 1+1+len(host)+2)
		buf[0] = 0x03 // Domain name
		buf[1] = byte(len(host))
		copy(buf[2:], []byte(host))
	}

	// 写入端口(网络字节序)
	buf[len(buf)-2], buf[len(buf)-1] = byte(port>>8), byte(port)
	return buf, nil
}

func newShadowsocksProxyClient(proxy *url.URL, upstreamDial Dial) (dial Dial, err error) {
	if proxy, err = decodedBase64EncodedURL(proxy); err != nil {
		return
	}
	if proxy.User == nil {
		err = errors.New("method and password is not available")
		return
	}
	var cipher ss.Cipher
	if password, ok := proxy.User.Password(); ok {
		method := proxy.User.Username()
		cipher, err = ss.PickCipher(method, nil, password)
		if err != nil {
			return
		}
	}

	conn, err := upstreamDial(context.Background(), "tcp", proxy.Host)
	if err != nil {
		return nil, err
	}
	conn = cipher.StreamConn(conn)
	dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		portI, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}
		addr, err := buildSSAddr(host, portI)
		if err != nil {
			return nil, err
		}
		_, err = conn.Write(addr)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	return
}
