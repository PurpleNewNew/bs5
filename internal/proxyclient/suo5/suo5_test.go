package suo5

import (
	"net/url"
	"testing"
)

func TestSuo5ClientDial(t *testing.T) {
	// Create test client
	proxyURL, _ := url.Parse("suo5://127.0.0.1:8089/suo5.jsp")
	conf, err := NewConfFromURL(proxyURL)
	if err != nil {
		t.Fatal(err)
	}

	client := &Suo5Client{
		Proxy: proxyURL,
		Conf:  conf,
	}

	// Test connection
	conn, err := client.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("restet"))
	if err != nil {
		return
	}
}
