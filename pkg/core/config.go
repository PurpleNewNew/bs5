package core

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/PurpleNewNew/bs5/internal/proxyclient"
	"github.com/PurpleNewNew/bs5/internal/rawhttp"
	"github.com/PurpleNewNew/bs5/pkg/netrans"
	"github.com/gobwas/glob"
	log "github.com/kataras/golog"
	utls "github.com/refraction-networking/utls"
)

type Suo5Config struct {
	Method           string         `json:"method"`
	Listen           string         `json:"listen"`
	Target           string         `json:"target"`
	NoAuth           bool           `json:"no_auth"`
	Username         string         `json:"username"`
	Password         string         `json:"password"`
	Mode             ConnectionType `json:"mode"`
	BufferSize       int            `json:"buffer_size"`
	Timeout          int            `json:"timeout"`
	Debug            bool           `json:"debug"`
	UpstreamProxy    []string       `json:"upstream_proxy"`
	RedirectURL      string         `json:"redirect_url"`
	RawHeader        []string       `json:"raw_header"`
	DisableHeartbeat bool           `json:"disable_heartbeat"`
	DisableGzip      bool           `json:"disable_gzip"`
	EnableCookieJar  bool           `json:"enable_cookiejar"`
	ExcludeDomain    []string       `json:"exclude_domain"`
	ForwardTarget    string         `json:"forward_target"`

	TestExit                string                               `json:"-"`
	ExcludeGlobs            []glob.Glob                          `json:"-"`
	Offset                  int                                  `json:"-"`
	Header                  http.Header                          `json:"-"`
	ProxyClient             proxyclient.Dial                     `json:"-"`
	OnRemoteConnected       func(e *ConnectedEvent)              `json:"-"`
	OnNewClientConnection   func(event *ClientConnectionEvent)   `json:"-"`
	OnClientConnectionClose func(event *ClientConnectCloseEvent) `json:"-"`
	GuiLog                  io.Writer                            `json:"-"`
}

func (s *Suo5Config) Parse() error {
	if err := s.parseExcludeDomain(); err != nil {
		return err
	}
	return s.parseHeader()
}

func (s *Suo5Config) parseExcludeDomain() error {
	s.ExcludeGlobs = make([]glob.Glob, 0)
	for _, domain := range s.ExcludeDomain {
		g, err := glob.Compile(domain)
		if err != nil {
			return err
		}
		s.ExcludeGlobs = append(s.ExcludeGlobs, g)
	}
	return nil
}

func (s *Suo5Config) parseHeader() error {
	s.Header = make(http.Header)
	for _, value := range s.RawHeader {
		if value == "" {
			continue
		}
		parts := strings.SplitN(value, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid header value %s", value)
		}
		s.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}

	if s.Header.Get("Referer") == "" {
		n := strings.LastIndex(s.Target, "/")
		if n == -1 {
			s.Header.Set("Referer", s.Target)
		} else {
			s.Header.Set("Referer", s.Target[:n+1])
		}
	}

	return nil
}

func (config *Suo5Config) Init(ctx context.Context) (*Suo5Client, error) {
	err := config.Parse()
	if err != nil {
		return nil, err
	}

	tr, err := newHTTPTransport(config)
	if err != nil {
		return nil, err
	}

	jar := newCookieJar(config)

	noTimeoutClient := &http.Client{
		Transport: tr.Clone(),
		Jar:       jar,
		Timeout:   0,
	}
	normalClient := &http.Client{
		Timeout:   time.Duration(config.Timeout) * time.Second,
		Jar:       jar,
		Transport: tr.Clone(),
	}

	var rawClient *rawhttp.Client
	if config.ProxyClient != nil {
		rawClient = newRawClient(config.ProxyClient.DialContext, 0)
	} else {
		rawClient = newRawClient(nil, 0)
	}

	log.Infof("header: %s", config.HeaderString())
	log.Infof("method: %s", config.Method)
	log.Infof("connecting to target %s", config.Target)
	result, offset, err := checkConnectMode(ctx, config)
	if err != nil {
		return nil, err
	}
	if config.Mode == AutoDuplex {
		config.Mode = result
		if result == FullDuplex {
			log.Infof("wow, you can run the proxy on FullDuplex mode")
		} else {
			log.Warnf("the target may behind a reverse proxy, fallback to HalfDuplex mode")
		}
	} else {
		if result == FullDuplex && config.Mode == HalfDuplex {
			log.Infof("the target support full duplex, you can try FullDuplex mode to obtain better performance")
		} else if result == HalfDuplex && config.Mode == FullDuplex {
			return nil, fmt.Errorf("the target doesn't support full duplex, you should use HalfDuplex or AutoDuplex mode")
		}
	}
	config.Offset = offset
	return &Suo5Client{
		Config:          config,
		NormalClient:    normalClient,
		NoTimeoutClient: noTimeoutClient,
		RawClient:       rawClient,
	}, nil
}

// newHTTPTransport creates and configures an http.Transport based on the Suo5Config.
func newHTTPTransport(config *Suo5Config) (*http.Transport, error) {
	if config.DisableGzip {
		log.Infof("disable gzip")
		config.Header.Set("Accept-Encoding", "identity")
	}

	if len(config.ExcludeDomain) != 0 {
		log.Infof("exclude domains: %v", config.ExcludeDomain)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS10,
			Renegotiation:      tls.RenegotiateOnceAsClient,
			InsecureSkipVerify: true,
		},
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.DialTimeout(network, addr, 5*time.Second)
			if err != nil {
				return nil, err
			}
			colonPos := strings.LastIndex(addr, ":")
			if colonPos == -1 {
				colonPos = len(addr)
			}
			hostname := addr[:colonPos]
			tlsConfig := &utls.Config{
				ServerName:         hostname,
				InsecureSkipVerify: true,
				Renegotiation:      utls.RenegotiateOnceAsClient,
				MinVersion:         utls.VersionTLS10,
			}
			uTlsConn := utls.UClient(conn, tlsConfig, utls.HelloRandomizedNoALPN)
			if err = uTlsConn.HandshakeContext(ctx); err != nil {
				_ = conn.Close()
				return nil, err
			}
			return uTlsConn, nil
		},
	}

	if len(config.UpstreamProxy) > 0 {
		proxies, err := proxyclient.ParseProxyURLs(config.UpstreamProxy)
		if err != nil {
			return nil, err
		}
		log.Infof("using upstream proxy %v", proxies)

		config.ProxyClient, err = proxyclient.NewClientChain(proxies)
		if err != nil {
			return nil, err
		}
		tr.DialContext = config.ProxyClient.DialContext
	}

	if config.RedirectURL != "" {
		_, err := url.Parse(config.RedirectURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redirect url, %s", err)
		}
		log.Infof("using redirect url %v", config.RedirectURL)
	}

	return tr, nil
}

// newCookieJar creates a cookie jar based on the Suo5Config.
func newCookieJar(config *Suo5Config) http.CookieJar {
	if config.EnableCookieJar {
		jar, _ := cookiejar.New(nil)
		return jar
	}
	// For PHP sites, automatically enable cookiejar for PHPSESSID to handle sessions.
	return NewSwitchableCookieJar([]string{"PHPSESSID"})
}

func (s *Suo5Config) HeaderString() string {
	ret := ""
	for k := range s.Header {
		ret += fmt.Sprintf("\n%s: %s", k, s.Header.Get(k))
	}
	return ret
}

func DefaultSuo5Config() *Suo5Config {
	return &Suo5Config{
		Method:           "POST",
		Listen:           "127.0.0.1:1111",
		Target:           "",
		NoAuth:           true,
		Username:         "",
		Password:         "",
		Mode:             "auto",
		BufferSize:       1024 * 320,
		Timeout:          10,
		Debug:            false,
		UpstreamProxy:    []string{},
		RedirectURL:      "",
		RawHeader:        []string{"User-Agent: Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.1.2.3"},
		DisableHeartbeat: false,
		DisableGzip:      false,
		EnableCookieJar:  false,
		ForwardTarget:    "",
	}
}

func checkConnectMode(ctx context.Context, config *Suo5Config) (ConnectionType, int, error) {
	// Use a context with a timeout for this check
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var rawClient *rawhttp.Client
	if config.ProxyClient != nil {
		rawClient = newRawClient(config.ProxyClient.DialContext, 0) // Timeout is handled by context
	} else {
		rawClient = newRawClient(nil, 0)
	}

	randLen := rander.Intn(1024)
	if randLen <= 32 {
		randLen += 32
	}
	data := RandString(randLen)
	ch := make(chan []byte, 1)
	ch <- []byte(data)

	// Use a goroutine to close the channel when the context is done
	go func() {
		select {
		case <-checkCtx.Done():
			close(ch)
		}
	}()

	req, err := http.NewRequestWithContext(checkCtx, config.Method, config.Target, netrans.NewChannelReader(ch))
	if err != nil {
		return Undefined, 0, err
	}
	req.Header = config.Header.Clone()
	req.Header.Set(HeaderKey, HeaderValueChecking)

	now := time.Now()
	resp, err := rawClient.Do(req)
	if err != nil {
		// Check if the error is due to context cancellation
		if ctxErr := checkCtx.Err(); ctxErr != nil {
			log.Warnf("connection mode check timed out: %v", ctxErr)
			// This is not a fatal error, we can assume HalfDuplex
			return HalfDuplex, 0, nil
		}
		return Undefined, 0, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Warnf("got error while closing body: %s", err)
		}
	}(resp.Body)

	duration := time.Since(now)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Warnf("got error while reading body: %s", err)
	}

	offset := strings.Index(string(body), data[:32])
	if offset == -1 {
		header, _ := httputil.DumpResponse(resp, false)
		log.Errorf("response are as follows:\n%s", string(header)+string(body))
		return Undefined, 0, fmt.Errorf("got unexpected body, remote server test failed")
	}
	log.Infof("got data offset, %d", offset)

	// If the request completed quickly, we can assume FullDuplex
	if duration < 3*time.Second {
		return FullDuplex, offset, nil
	} else {
		return HalfDuplex, offset, nil
	}
}
