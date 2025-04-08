package suo5

import (
	"context"
	"fmt"
	"github.com/PurpleNewNew/bs5/pkg/core"
	"net"
	"net/url"
	"strings"
	"time"
)

type Suo5Client struct {
	Proxy *url.URL
	Conf  *Suo5Conf
}

type Suo5Conf struct {
	*core.Suo5Client
	*core.Suo5Config
}

// NewConfFromURL 从URL中解析用户名密码生成配置
func NewConfFromURL(proxyURL *url.URL) (*Suo5Conf, error) {
	scheme := "http"
	switch strings.ToLower(proxyURL.Scheme) {
	case "suo5":
		scheme = "http"
	case "suo5s":
		scheme = "https"
	default:
		return nil, fmt.Errorf("unsupported scheme: %s", proxyURL.Scheme)
	}

	// 使用这些值构建配置
	config := core.DefaultSuo5Config()
	config.Target = fmt.Sprintf("%s://%s%s", scheme, proxyURL.Host, proxyURL.Path)
	client, err := config.Init()
	if err != nil {
		return nil, err
	}

	suo5Conf := &Suo5Conf{
		Suo5Config: config,
		Suo5Client: client,
	}

	return suo5Conf, nil
}

// Dial 实现了Client接口
func (c *Suo5Client) Dial(network, address string) (net.Conn, error) {
	// 创建一个新的 suo5Conn 连接
	//conn, err := c.conf.Upstream(network, address)
	//if err != nil {
	//	return nil, err
	//}

	suo5Conn := &suo5Conn{
		Suo5Conn: core.NewSuo5Conn(context.Background(), c.Conf.Suo5Client),
		Suo5Conf: c.Conf,
	}

	// 发送连接请求
	if err := suo5Conn.connect(address); err != nil {
		return nil, err
	}

	return suo5Conn, nil
}

// suo5Conn 实现了net.Conn接口
type suo5Conn struct {
	*core.Suo5Conn
	*Suo5Conf
}

func (conn *suo5Conn) connect(address string) error {
	return conn.Suo5Conn.Connect(address)
}

func (conn *suo5Conn) LocalAddr() net.Addr {
	return nil
}

func (conn *suo5Conn) RemoteAddr() net.Addr {
	return nil
}

func (conn *suo5Conn) SetDeadline(t time.Time) error {
	return nil
}

func (conn *suo5Conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (conn *suo5Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
