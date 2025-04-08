package httpproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	Proxy        url.URL
	TLSConfig    *tls.Config
	UpstreamDial func(ctx context.Context, network, address string) (net.Conn, error)
}

func NewClient(proxy url.URL, dial func(ctx context.Context, network, address string) (net.Conn, error)) *Client {
	return &Client{proxy, nil, dial}
}

func (client *Client) Dial(ctx context.Context, network, address string) (conn net.Conn, err error) {
	switch strings.ToUpper(client.Proxy.Scheme) {
	case "HTTP", "HTTPS":
	default:
		err = errors.New("Proxy URL Scheme not HTTP or HTTPS")
		return
	}

	if conn, err = client.connect(network, client.Proxy.Host); err != nil {
		return
	}

	var request *http.Request
	var response *http.Response

	if request, err = client.newRequest(address); err != nil {
		conn.Close()
		return
	}
	if err = request.Write(conn); err != nil {
		conn.Close()
		return
	}
	if response, err = http.ReadResponse(bufio.NewReader(conn), request); err != nil {
		conn.Close()
		return
	}
	if response.StatusCode != http.StatusOK {
		err = conn.Close()
	}
	return
}

func (client *Client) connect(network, address string) (conn net.Conn, err error) {
	conn, err = client.UpstreamDial(context.Background(), network, address)
	if err != nil {
		return
	}
	if strings.ToUpper(client.Proxy.Scheme) == "HTTPS" {
		tlsConn := tls.Client(conn, client.TLSConfig)
		if err = tlsConn.Handshake(); err != nil {
			tlsConn.Close()
			return
		}
		conn = tlsConn
	}
	return
}

func (client *Client) newRequest(address string) (*http.Request, error) {
	//remoteHost, _, err := net.SplitHostPort(address)
	//if err != nil {
	//	return nil, err
	//}
	request, err := http.NewRequest(http.MethodConnect, "", nil)
	if err != nil {
		return nil, err
	}
	request.RequestURI = address
	request.URL.Host = address
	setBasicAuth(request, client.Proxy.User)
	return request, nil
}

func setBasicAuth(request *http.Request, user *url.Userinfo) {
	if user == nil {
		return
	}
	if password, ok := user.Password(); ok {
		username := user.Username()
		request.Header.Set(
			authorization,
			encodeBasicAuth(username, password),
		)
	}
}
