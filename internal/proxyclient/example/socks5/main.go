package main

import (
	"context"
	"fmt"
	"github.com/PurpleNewNew/bs5/internal/proxyclient"
	"log"
	"net"
	"net/url"
	"os"

	"github.com/things-go/go-socks5"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Printf("Usage: %s <proxy-url> <listen-addr>\n", os.Args[0])
		fmt.Printf("Example: %s http://127.0.0.1:8080 :1080\n", os.Args[0])
		os.Exit(1)
	}

	proxyURL, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Printf("Invalid proxy URL: %v\n", err)
		os.Exit(1)
	}

	listenAddr := os.Args[2]

	// 创建代理客户端
	dial, err := proxyclient.NewClient(proxyURL)
	if err != nil {
		fmt.Printf("Failed to create proxy client: %v\n", err)
		os.Exit(1)
	}

	// 创建SOCKS5服务器配置
	server := socks5.NewServer(
		socks5.WithDial(func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dial(ctx, network, addr)
		}),
	)

	fmt.Printf("SOCKS5 server listening on %s\n", listenAddr)
	if err := server.ListenAndServe("tcp", listenAddr); err != nil {
		log.Fatal(err)
	}
}
