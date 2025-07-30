package core

import (
	"bytes"
	"context"
	"fmt"
	netrans2 "github.com/PurpleNewNew/bs5/pkg/netrans"
	log "github.com/kataras/golog"
	"github.com/pkg/errors"
	"io"
	"net"
	"net/http"
	"strconv"
)

var (
	ErrHostUnreachable = errors.New("host unreachable")
	ErrDialFailed      = errors.New("dial failed")
	ErrConnRefused     = errors.New("connection refused")
)

// 用于创建一个Suo5Conn
func NewSuo5Conn(ctx context.Context, client *Suo5Client) *Suo5Conn {
	return &Suo5Conn{
		ctx:        ctx,
		Suo5Client: client,
	}
}

// Suo5Conn 结构体
type Suo5Conn struct {
	io.ReadWriteCloser
	ctx context.Context
	*Suo5Client
}

// 连接方法，
func (suo *Suo5Conn) Connect(address string) error {
	id := RandString(8)
	var req *http.Request
	var resp *http.Response
	var err error
	host, port, _ := net.SplitHostPort(address)
	uport, _ := strconv.Atoi(port)
	dialData := BuildBody(NewActionCreate(id, host, uint16(uport), suo.Config.RedirectURL))
	ch, chWR := netrans2.NewChannelWriteCloser(suo.ctx)

	baseHeader := suo.Config.Header.Clone()

	if suo.Config.Mode == FullDuplex {
		body := netrans2.MultiReadCloser(
			io.NopCloser(bytes.NewReader(dialData)),
			io.NopCloser(netrans2.NewChannelReader(ch)),
		)
		req, _ = http.NewRequestWithContext(suo.ctx, suo.Config.Method, suo.Config.Target, body)
		baseHeader.Set(HeaderKey, HeaderValueFull)
		req.Header = baseHeader
		resp, err = suo.RawClient.Do(req)
	} else {
		req, _ = http.NewRequestWithContext(suo.ctx, suo.Config.Method, suo.Config.Target, bytes.NewReader(dialData))
		baseHeader.Set(HeaderKey, HeaderValueHalf)
		req.Header = baseHeader
		resp, err = suo.NoTimeoutClient.Do(req)
	}
	if err != nil {
		log.Debugf("request error to target, %s", err)
		return errors.Wrap(ErrHostUnreachable, err.Error())
	}

	if resp.Header.Get("Set-Cookie") != "" && suo.Config.EnableCookieJar {
		log.Infof("update cookie with %s", resp.Header.Get("Set-Cookie"))
	}

	// skip offset
	if suo.Config.Offset > 0 {
		log.Debugf("skipping offset %d", suo.Config.Offset)
		_, err = io.CopyN(io.Discard, resp.Body, int64(suo.Config.Offset))
		if err != nil {
			log.Errorf("failed to skip offset, %s", err)
			return errors.Wrap(ErrDialFailed, err.Error())
		}
	}
	fr, err := netrans2.ReadFrame(resp.Body)
	if err != nil {
		log.Errorf("failed to read response frame, may be the target has load balancing?")

		return errors.Wrap(ErrHostUnreachable, err.Error())
	}
	log.Debugf("recv dial response from server: length: %d", fr.Length)

	serverData, err := Unmarshal(fr.Data)
	if err != nil {
		log.Errorf("failed to process frame, %v", err)
		return errors.Wrap(ErrHostUnreachable, err.Error())
	}
	status := serverData["s"]
	if len(status) != 1 || status[0] != 0x00 {
		return errors.Wrap(ErrHostUnreachable, fmt.Sprintf("failed to dial, status: %v", status))
	}

	var streamRW io.ReadWriteCloser
	if suo.Config.Mode == FullDuplex {
		streamRW = NewFullChunkedReadWriter(id, chWR, resp.Body)
	} else {
		streamRW = NewHalfChunkedReadWriter(suo.ctx, id, suo.NormalClient, suo.Config.Method, suo.Config.Target,
			resp.Body, baseHeader, suo.Config.RedirectURL)
	}

	if !suo.Config.DisableHeartbeat {
		streamRW = NewHeartbeatRW(streamRW.(RawReadWriteCloser), id, suo.Config.RedirectURL)
	}

	suo.ReadWriteCloser = streamRW
	return nil
}
