package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/PurpleNewNew/bs5/internal/proxyclient"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Printf("Usage: %s <proxy-url> <target-host> <target-port>\n", os.Args[0])
		fmt.Printf("Example: %s http://127.0.0.1:8080 example.com 80\n", os.Args[0])
		os.Exit(1)
	}

	proxyURL, err := url.Parse(os.Args[1])
	if err != nil {
		fmt.Printf("Invalid proxy URL: %v\n", err)
		os.Exit(1)
	}

	targetHost := os.Args[2]
	targetPort := os.Args[3]

	// 创建代理客户端
	dial, err := proxyclient.NewClient(proxyURL)
	if err != nil {
		fmt.Printf("Failed to create proxy client: %v\n", err)
		os.Exit(1)
	}

	// 连接目标服务器
	conn, err := dial(context.Background(), "tcp", fmt.Sprintf("%s:%s", targetHost, targetPort))
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Connected to %s:%s\n", targetHost, targetPort)

	// 创建双向数据传输
	go io.Copy(conn, os.Stdin)
	io.Copy(os.Stdout, conn)
}
