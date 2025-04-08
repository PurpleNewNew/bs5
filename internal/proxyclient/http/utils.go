package httpproxy

import (
	"encoding/base64"
	"net"
	"net/url"
	"strings"
)

var (
	authenticate  = "Proxy-Authenticate"
	authorization = "Proxy-Authorization"
)

func encodeBasicAuth(username, password string) string {
	const prefix = "Basic "
	return prefix + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

func decodeBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return
	}
	auth = auth[len(prefix):]
	if decoded, err := base64.StdEncoding.DecodeString(auth); err == nil {
		splitted := strings.Split(string(decoded), ":")
		if len(splitted) == 2 {
			return splitted[0], splitted[1], true
		}
	}
	return
}

func urlToRemoteAddress(url *url.URL) string {
	if url.Port() != "" {
		return url.Host
	}
	var port string
	switch strings.ToUpper(url.Scheme) {
	case "HTTP":
		port = "80"
	case "HTTPS":
		port = "443"
	}
	return net.JoinHostPort(url.Hostname(), port)
}
