package rawhttp

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func getTestHttpServer(timeout time.Duration) *httptest.Server {
	var ts *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/rawhttp", func(writer http.ResponseWriter, request *http.Request) {
		time.Sleep(timeout)
	})
	ts = httptest.NewServer(mux)
	return ts
}

// run with go test -timeout 45s -run ^TestDialDefaultTimeout$ github.com/projectdiscovery/rawhttp
func TestDialDefaultTimeout(t *testing.T) {
	timeout := 30 * time.Second
	ts := getTestHttpServer(45 * time.Second)
	defer ts.Close()

	startTime := time.Now()
	client := NewClient(DefaultOptions)
	_, err := client.DoRaw("GET", ts.URL, "/rawhttp", nil, nil)
	if !strings.ContainsAny(err.Error(), "i/o timeout") || time.Now().Before(startTime.Add(timeout)) {
		t.Error("default timeout error")
	}
}

func TestDialWithCustomTimeout(t *testing.T) {
	timeout := 5 * time.Second
	ts := getTestHttpServer(10 * time.Second)
	defer ts.Close()

	startTime := time.Now()
	client := NewClient(DefaultOptions)
	options := DefaultOptions
	options.Timeout = timeout
	_, err := client.DoRawWithOptions("GET", ts.URL, "/rawhttp", nil, nil, options)
	if !strings.ContainsAny(err.Error(), "i/o timeout") || time.Now().Before(startTime.Add(timeout)) {
		t.Error("custom timeout error")
	}
}

func TestSimpleRequest(t *testing.T) {
	options := &Options{
		Timeout:                0 * time.Second,
		FollowRedirects:        false,
		MaxRedirects:           0,
		AutomaticHostHeader:    true,
		AutomaticContentLength: true,
		ForceReadAllBody:       false,
	}
	client := NewClient(options)
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Printf("status: %d, body length: %d\n", resp.StatusCode, len(data))
}
