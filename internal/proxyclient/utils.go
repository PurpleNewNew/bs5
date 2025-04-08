package proxyclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io/ioutil"
	"net"
	"net/url"
	"strings"
	"time"
)

func ParseProxyURLs(proxyURL []string) ([]*url.URL, error) {
	var proxies []*url.URL
	for _, u := range proxyURL {
		proxy, err := url.Parse(u)
		if err != nil {
			return nil, err
		}
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}

func WrapDialerContext(dialer func(network, address string) (net.Conn, error)) Dial {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer(network, address)
	}
}

func normalizeLink(proxy url.URL) *url.URL {
	switch strings.ToUpper(proxy.Path) {
	case "DIRECT", "REJECT", "BLACKHOLE":
		proxy = url.URL{Scheme: proxy.Path}
	}

	proxy.Scheme = strings.ToUpper(proxy.Scheme)
	query := proxy.Query()
	for name, value := range query {
		query[strings.ToLower(name)] = value
	}
	proxy.RawQuery = query.Encode()
	return &proxy
}

func DialWithTimeout(timeout time.Duration) Dial {
	dialer := net.Dialer{Timeout: timeout}
	return dialer.DialContext
}

func decodedBase64EncodedURL(proxy *url.URL) (*url.URL, error) {
	if proxy.Scheme == "" && proxy.Host == "" {
		return proxy, nil
	}
	content, err := base64.StdEncoding.DecodeString(proxy.Host)
	if err == nil {
		return proxy.Parse(proxy.Scheme + "://" + string(content))
	}
	return proxy, nil
}

func tlsConfigByProxyURL(proxy *url.URL) (conf *tls.Config) {
	query := proxy.Query()
	conf = &tls.Config{
		ServerName:         query.Get("tls-domain"),
		InsecureSkipVerify: query.Get("tls-insecure-skip-verify") == "true",
	}
	if conf.ServerName == "" {
		conf.ServerName = proxy.Host
	}
	if caFile := query.Get("tls-ca-file"); caFile != "" {
		certPool := x509.NewCertPool()
		pem, err := ioutil.ReadFile(caFile)
		if err != nil {
			return
		}
		if !certPool.AppendCertsFromPEM(pem) {
			return
		}
		conf.RootCAs = certPool
		conf.ClientCAs = certPool
	}
	return
}
