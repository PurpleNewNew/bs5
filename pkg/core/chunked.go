package core

import (
	"bytes"
	"context"
	"fmt"
	"github.com/OmagariHare/bs5/pkg/netrans"
	log "github.com/kataras/golog"
	"io"
	"net/http"
	"sync"
)

// 分块读写结构体
type fullChunkedReadWriter struct {
	id         string
	reqBody    io.WriteCloser
	serverResp io.ReadCloser
	once       sync.Once

	readBuf  bytes.Buffer
	readTmp  []byte
	writeTmp []byte
}

// NewFullChunkedReadWriter 全双工读写流
func NewFullChunkedReadWriter(id string, reqBody io.WriteCloser, serverResp io.ReadCloser) io.ReadWriteCloser {
	rw := &fullChunkedReadWriter{
		id:         id,
		reqBody:    reqBody,
		serverResp: serverResp,
		readBuf:    bytes.Buffer{},
		readTmp:    make([]byte, 16*1024), //16KB 的字节切片，用于存储读的数据
		writeTmp:   make([]byte, 8*1024),  //8KB 的字节切片，用于存储写的数据
	}
	return rw
}

// Read 全双工读操作
func (s *fullChunkedReadWriter) Read(p []byte) (n int, err error) {
	// 如果readBuf中有剩余数据，直接读取
	if s.readBuf.Len() != 0 {
		return s.readBuf.Read(p)
	}
	// 读取帧
	fr, err := netrans.ReadFrame(s.serverResp)
	if err != nil {
		return 0, err
	}
	// 从帧中解析出k-v数据
	m, err := Unmarshal(fr.Data)
	if err != nil {
		return 0, err
	}
	action := m["ac"]
	if len(action) != 1 {
		return 0, fmt.Errorf("invalid action when read %v", action)
	}
	switch action[0] {
	case ActionData:
		data := m["dt"]
		s.readBuf.Reset()
		s.readBuf.Write(data)
		return s.readBuf.Read(p)
	case ActionDelete:
		return 0, io.EOF
	default:
		return 0, fmt.Errorf("unpected action when read %v", action)
	}
}

// Write 生成NewActionData=1的帧，调用实际写操作
func (s *fullChunkedReadWriter) Write(p []byte) (n int, err error) {
	log.Debugf("write socket data, length: %d", len(p))
	body := BuildBody(NewActionData(s.id, p, ""))
	return s.WriteRaw(body)
}

// WriteRaw 实际写操作
func (s *fullChunkedReadWriter) WriteRaw(p []byte) (n int, err error) {
	return s.reqBody.Write(p)
}

// Close 关闭连接
func (s *fullChunkedReadWriter) Close() error {
	s.once.Do(func() {
		defer func(reqBody io.WriteCloser) {
			_ = reqBody.Close()
		}(s.reqBody)
		body := BuildBody(NewDelete(s.id, ""))
		_, _ = s.reqBody.Write(body)
		_ = s.serverResp.Close()
	})
	return nil
}

// 半双工读写流结构体
type halfChunkedReadWriter struct {
	ctx        context.Context
	id         string
	client     *http.Client
	method     string
	target     string
	serverResp io.ReadCloser
	once       sync.Once
	chunked    bool
	baseHeader http.Header
	redirect   string

	readBuf  bytes.Buffer
	readTmp  []byte
	writeTmp []byte
}

// NewHalfChunkedReadWriter 半双工读写流, 用发送请求的方式模拟写
func NewHalfChunkedReadWriter(ctx context.Context, id string, client *http.Client, method, target string,
	serverResp io.ReadCloser, baseHeader http.Header, redirect string) io.ReadWriteCloser {
	return &halfChunkedReadWriter{
		ctx:        ctx,
		id:         id,
		client:     client,
		method:     method,
		target:     target,
		serverResp: serverResp,
		readBuf:    bytes.Buffer{},
		readTmp:    make([]byte, 16*1024),
		writeTmp:   make([]byte, 8*1024),
		baseHeader: baseHeader,
		redirect:   redirect,
	}
}

// Read 半双工读操作
func (s *halfChunkedReadWriter) Read(p []byte) (n int, err error) {
	if s.readBuf.Len() != 0 {
		return s.readBuf.Read(p)
	}
	fr, err := netrans.ReadFrame(s.serverResp)
	if err != nil {
		return 0, err
	}
	m, err := Unmarshal(fr.Data)
	if err != nil {
		return 0, err
	}
	action := m["ac"]
	if len(action) != 1 {
		return 0, fmt.Errorf("invalid action when read %v", action)
	}
	switch action[0] {
	case ActionData:
		data := m["dt"]
		s.readBuf.Reset()
		s.readBuf.Write(data)
		return s.readBuf.Read(p)
	case ActionDelete:
		return 0, io.EOF
	case ActionHeartbeat:
		return 0, nil
	default:
		return 0, fmt.Errorf("unpected action when read %v", action)
	}
}

// Write 半双工写操作
func (s *halfChunkedReadWriter) Write(p []byte) (n int, err error) {
	body := BuildBody(NewActionData(s.id, p, s.redirect))
	log.Debugf("send request, length: %d", len(body))
	return s.WriteRaw(body)
}

// WriteRaw 实际半双工写操作
func (s *halfChunkedReadWriter) WriteRaw(p []byte) (n int, err error) {
	req, err := http.NewRequestWithContext(s.ctx, s.method, s.target, bytes.NewReader(p))
	if err != nil {
		return 0, err
	}
	if s.chunked {
		req.ContentLength = -1
	} else {
		req.ContentLength = int64(len(p))
	}
	req.Header = s.baseHeader.Clone()
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return len(p), nil
	} else {
		return 0, fmt.Errorf("unexpected status of %d", resp.StatusCode)
	}
}

// Close 关闭连接
func (s *halfChunkedReadWriter) Close() error {
	s.once.Do(func() {
		body := BuildBody(NewDelete(s.id, s.redirect))
		req, err := http.NewRequestWithContext(s.ctx, s.method, s.target, bytes.NewReader(body))
		if err != nil {
			log.Error(err)
			return
		}
		req.Header = s.baseHeader.Clone()
		resp, err := s.client.Do(req)
		if err != nil {
			log.Errorf("send close error: %v", err)
			return
		}
		_ = resp.Body.Close()
		_ = s.serverResp.Close()
	})
	return nil
}
