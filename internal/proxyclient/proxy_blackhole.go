package proxyclient

import (
	"context"
	"errors"
	"net"
	"net/url"
	"time"
)

func newBlackholeProxyClient(_ *url.URL, _ Dial) (dial Dial, err error) {
	dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		return blackholeConn{}, nil
	}
	return
}

type blackholeConn struct{}

func (blackholeConn) Read([]byte) (int, error)         { return 0, nil }
func (blackholeConn) Write(buffer []byte) (int, error) { return len(buffer), nil }
func (blackholeConn) Close() error                     { return nil }
func (blackholeConn) LocalAddr() net.Addr              { return nil }
func (blackholeConn) RemoteAddr() net.Addr             { return nil }
func (blackholeConn) SetDeadline(time.Time) error      { return errors.New("unsupported") }
func (blackholeConn) SetReadDeadline(time.Time) error  { return errors.New("unsupported") }
func (blackholeConn) SetWriteDeadline(time.Time) error { return errors.New("unsupported") }
