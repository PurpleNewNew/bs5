package httpproxy

import (
	"errors"
	"io"
	"net"
	"net/http"
)

var (
	responseEstablished = http.Response{
		StatusCode: 200,
		Status:     "Connection Established",
	}
	responseUnauthorized = http.Response{
		StatusCode: http.StatusUnauthorized,
		Status:     http.StatusText(http.StatusUnauthorized),
		Header:     http.Header{authenticate: []string{"Basic"}},
	}
)

type Handler struct {
	Auth        func(username, password string) bool
	Dial        func(network, address string) (net.Conn, error)
	HandleError func(error, *http.Request)
	client      *http.Client
}

func Serve(listener net.Listener, dial func(network, address string) (net.Conn, error)) {
	http.Serve(listener, Handler{Dial: dial})
}

func (h Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h.basicAuth(writer, request) {
		return
	}
	var err error
	if request.Method == http.MethodConnect {
		err = h.handleConnect(writer, request)
	} else {
		err = h.handleNormal(writer, request)
	}
	if err != nil && h.HandleError != nil {
		go h.HandleError(err, request)
	}
}

func (h Handler) handleConnect(writer http.ResponseWriter, request *http.Request) error {
	hijacker, ok := writer.(http.Hijacker)
	if !ok {
		return errors.New("can't cast to Hijacker")
	}
	localConn, buffer, err := hijacker.Hijack()
	if err != nil {
		return err
	}
	if err := responseEstablished.Write(buffer); err != nil {
		return err
	}
	if err := buffer.Flush(); err != nil {
		return err
	}
	remoteConn, err := h.Dial("tcp", urlToRemoteAddress(request.URL))
	if err != nil {
		return err
	}
	go io.Copy(localConn, remoteConn)
	go io.Copy(remoteConn, localConn)
	return nil
}

func (h Handler) handleNormal(writer http.ResponseWriter, request *http.Request) error {
	response, err := h.request(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	header := writer.Header()
	for name, values := range response.Header {
		header[name] = values
	}
	if _, err := io.Copy(writer, response.Body); err != nil {
		return err
	}
	return nil
}

func (h Handler) request(request *http.Request) (*http.Response, error) {
	if h.client == nil {
		h.client = &http.Client{
			Transport:     &http.Transport{Dial: h.Dial},
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return nil },
		}
	}
	request.RequestURI = ""
	return h.client.Do(request)
}

func (h Handler) basicAuth(writer http.ResponseWriter, request *http.Request) (ok bool) {
	if h.Auth == nil {
		return
	}
	auth := request.Header.Get(authorization)
	if username, password, ok := decodeBasicAuth(auth); ok {
		ok = h.Auth(username, password)
	} else {
		responseUnauthorized.Write(writer)
	}
	return
}
