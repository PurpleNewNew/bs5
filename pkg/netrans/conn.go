package netrans

import (
	"net"
	"time"
)

// TimeoutConn 是一个 net.Conn 读写 timeout
type TimeoutConn struct {
	net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewTimeoutConn 创建一个 TimeoutConn
func NewTimeoutConn(conn net.Conn, readTimeout, writeTimeout time.Duration) net.Conn {
	return &TimeoutConn{
		Conn:         conn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

// Read 读操作
func (c *TimeoutConn) Read(b []byte) (n int, err error) {
	if c.readTimeout > 0 {
		err := c.Conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		if err != nil {
			return 0, err
		}
	}
	return c.Conn.Read(b)
}

// Write 写操作
func (c *TimeoutConn) Write(b []byte) (n int, err error) {
	if c.writeTimeout > 0 {
		err := c.Conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		if err != nil {
			return 0, err
		}
	}
	return c.Conn.Write(b)
}
