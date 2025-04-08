package proxyclient

import (
	"net/url"
	"testing"
	"time"
)

func TestNewClientWithDial(t *testing.T) {
	proxy, _ := url.Parse("ss://aes-256-gcm:sangfor@123@127.0.0.1:10086")
	client, _ := NewClient(proxy)
	conn, err := client.Dial("tcp", "127.0.0.1:5002")
	if err != nil {
		panic(err)
	}
	_, err = conn.Write([]byte("sadfafdasff"))
	if err != nil {
		panic(err)
		return
	}

	time.Sleep(1 * time.Second)
}
