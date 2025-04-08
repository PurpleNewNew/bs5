package neoreg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// 常量定义
const (
	cmdData          = 1
	cmdCommand       = 2
	cmdMark          = 3
	cmdStatus        = 4
	cmdError         = 5
	cmdIP            = 6
	cmdPort          = 7
	cmdRedirectURL   = 8
	cmdForceRedirect = 9

	minKeyLen  = 28
	blvHeadLen = 9

	// 命令相关常量
	cmdConnect    = "CONNECT"
	cmdDisconnect = "DISCONNECT"
	cmdForward    = "FORWARD"
	cmdRead       = "READ"
	statusOK      = "OK"
)

var (
	DefaultTimeout        = 5 * time.Second
	DefaultMaxRetry       = 10
	DefaultInterval       = 100 * time.Millisecond
	DefaultReadBufferSize = 32 * 1024
	saltPrefix            = []byte("11f271c6lm0e9ypkptad1uv6e1ut1fu0pt4xillz1w9bbs2gegbv89z9gca9d6tbk025uvgjfr331o0szln")
	BASE64CHARS           = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
)

var defaultHeaders = map[string]string{
	"Accept-Encoding": "gzip, deflate",
	"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Content-Type":    "application/octet-stream",
}

// NeoregClient 实现了Client接口
type NeoregClient struct {
	Proxy *url.URL
	Conf  *NeoregConf
}

// NeoregConf 配置结构
type NeoregConf struct {
	Dial     func(ctx context.Context, network, address string) (net.Conn, error)
	Protocol string // http/https

	EncodeMap map[byte]byte
	DecodeMap map[byte]byte

	Key  string
	Rand *NeoregRand

	blvOffset int32 // 对应Python中的BLV_L_OFFSET

	Timeout        time.Duration
	MaxRetry       int
	Interval       time.Duration
	ReadBufferSize int
}

// NewConfFromURL 从URL中解析用户名密码生成配置
func NewConfFromURL(proxyURL *url.URL) (*NeoregConf, error) {
	if proxyURL.User == nil {
		return nil, errors.New("username and password required in URL")
	}

	scheme := "http"
	switch strings.ToLower(proxyURL.Scheme) {
	case "neoreg":
		scheme = "http"
	case "neoregs":
		scheme = "https"
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", proxyURL.Scheme)
	}

	key := proxyURL.User.Username()
	mt := NewNeoregRand(key)
	encodeMap, decodeMap, blvOffset := generateMaps(mt)

	query := proxyURL.Query()
	conf := &NeoregConf{
		Dial:      (&net.Dialer{}).DialContext,
		Protocol:  scheme,
		EncodeMap: encodeMap,
		DecodeMap: decodeMap,
		Key:       key,
		Rand:      mt,
		blvOffset: blvOffset,

		Timeout:        DefaultTimeout,
		MaxRetry:       DefaultMaxRetry,
		Interval:       DefaultInterval,
		ReadBufferSize: DefaultReadBufferSize,
	}

	if v := query.Get("timeout"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			conf.Timeout = d
		}
	}
	if v := query.Get("retry"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			conf.MaxRetry = n
		}
	}
	if v := query.Get("interval"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			conf.Interval = d
		}
	}
	if v := query.Get("buffer_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			conf.ReadBufferSize = n
		}
	}

	return conf, nil
}

func (c *NeoregClient) Dial(network, address string) (net.Conn, error) {
	url := fmt.Sprintf("%s://%s%s", c.Conf.Protocol, c.Proxy.Host, c.Proxy.Path)

	nconn := &neoregConn{
		url:    url,
		mask:   randMask(),
		config: c.Conf,
	}

	// 建立目标连接
	if err := nconn.connect(address); err != nil {
		return nil, err
	}

	return nconn, nil
}

// neoregConn 实现了net.Conn接口
type neoregConn struct {
	net.Conn
	url    string
	mask   []byte
	closed bool
	config *NeoregConf

	// 读取缓冲区相关
	readBuf    []byte // 固定大小的循环缓冲区
	readStart  int    // 缓冲区读取位置
	readEnd    int    // 缓冲区写入位置
	readErr    error
	readClosed bool
	readChan   chan struct{} // 用于通知有新数据到达

	// HTTP客户端
	client *http.Client
}

func (c *neoregConn) connect(address string) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	// 设置HTTP客户端
	c.client = &http.Client{
		Timeout: c.config.Timeout,
	}

	info := map[int][]byte{
		cmdCommand: []byte(cmdConnect),
		cmdMark:    c.mask,
		cmdIP:      []byte(host),
		cmdPort:    []byte(port),
	}

	// 带重试的请求
	var resp map[int][]byte
	for retry := 0; retry < c.config.MaxRetry; retry++ {
		resp, err = c.request(info)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(retry*100) * time.Millisecond)
	}
	if err != nil {
		return err
	}

	if !bytes.Equal(resp[cmdStatus], []byte(statusOK)) {
		return errors.New("connect failed")
	}

	c.readBuf = make([]byte, c.config.ReadBufferSize)
	c.readChan = make(chan struct{}, 1)

	go c.readLoop()

	return nil
}

func (c *neoregConn) readLoop() {
	info := map[int][]byte{
		cmdCommand: []byte(cmdRead),
		cmdMark:    c.mask,
	}

	for !c.closed {
		resp, err := c.request(info)
		if err != nil {
			if !c.closed {
				c.readErr = err
				c.readClosed = true
				close(c.readChan)
			}
			return
		}

		if bytes.Equal(resp[cmdStatus], []byte(statusOK)) {
			if len(resp[cmdData]) > 0 {
				// 确保缓冲区有足够空间
				if len(resp[cmdData]) > len(c.readBuf) {
					// 如果数据大于缓冲区，扩展缓冲区
					newBuf := make([]byte, len(resp[cmdData])*2)
					n := copy(newBuf, c.readBuf[c.readStart:c.readEnd])
					c.readBuf = newBuf
					c.readStart = 0
					c.readEnd = n
				}

				available := len(c.readBuf) - c.readEnd
				if available < len(resp[cmdData]) {
					copy(c.readBuf, c.readBuf[c.readStart:c.readEnd])
					c.readEnd -= c.readStart
					c.readStart = 0
				}

				n := copy(c.readBuf[c.readEnd:], resp[cmdData])
				c.readEnd += n

				select {
				case c.readChan <- struct{}{}:
				default:
				}
			} else {
				time.Sleep(c.config.Interval)
			}
		} else {
			if !c.closed {
				c.readErr = errors.New("read failed")
				c.readClosed = true
				close(c.readChan)
			}
			return
		}
	}
}

func (c *neoregConn) Read(b []byte) (n int, err error) {
	if c.readClosed {
		return 0, c.readErr
	}

	if c.readStart < c.readEnd {
		n = copy(b, c.readBuf[c.readStart:c.readEnd])
		c.readStart += n
		return n, nil
	}

	_, ok := <-c.readChan
	if !ok {
		return 0, c.readErr
	}

	n = copy(b, c.readBuf[c.readStart:c.readEnd])
	c.readStart += n
	return n, nil
}

func (c *neoregConn) Write(b []byte) (n int, err error) {
	info := map[int][]byte{
		cmdCommand: []byte(cmdForward),
		cmdMark:    c.mask,
		cmdData:    b,
	}

	resp, err := c.request(info)
	if err != nil {
		return 0, err
	}

	if bytes.Equal(resp[cmdStatus], []byte(statusOK)) {
		return len(b), nil
	}
	return 0, errors.New("write failed")
}

func (c *neoregConn) Close() error {
	if !c.closed {
		c.closed = true

		// 发送断开连接请求
		info := map[int][]byte{
			cmdCommand: []byte(cmdDisconnect),
			cmdMark:    c.mask,
		}

		// 不需要重试，尝试一次即可
		_, _ = c.request(info)

		// 关闭读取通道
		if !c.readClosed {
			c.readClosed = true
			close(c.readChan)
		}
	}
	return nil
}

// 内部辅助方法
func (c *neoregConn) request(info map[int][]byte) (map[int][]byte, error) {
	data := encodeBody(info, c.config)

	req, err := http.NewRequest("POST", c.url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return decodeBody(body, c.config), nil
}
