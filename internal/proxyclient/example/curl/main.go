package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PurpleNewNew/bs5/internal/proxyclient"
)

func main() {
	// 定义命令行参数
	var (
		insecure bool
		proxy    string
	)

	flag.BoolVar(&insecure, "k", false, "允许不安全的SSL连接")
	flag.StringVar(&proxy, "proxy", "", "使用代理 (e.g., http://proxy:8080)")
	flag.Parse()

	// 检查剩余参数（URL）
	args := flag.Args()
	if len(args) != 1 {
		fmt.Printf("Usage: %s [options] <url>\n", os.Args[0])
		fmt.Printf("Options:\n")
		fmt.Printf("  -k\t\t允许不安全的SSL连接\n")
		fmt.Printf("  --proxy string\t使用代理 (e.g., http://proxy:8080)\n")
		fmt.Printf("\nExample: %s -k --proxy http://127.0.0.1:8080 https://example.com\n", os.Args[0])
		os.Exit(1)
	}

	targetURL := args[0]

	// 创建代理客户端
	var dial proxyclient.Dial
	var err error

	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			fmt.Printf("Invalid proxy URL: %v\n", err)
			os.Exit(1)
		}

		dial, err = proxyclient.NewClient(proxyURL)
		if err != nil {
			fmt.Printf("Failed to create proxy client: %v\n", err)
			os.Exit(1)
		}
	}

	// 创建HTTP客户端
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
	}

	if dial != nil {
		transport.DialContext = dial
	}

	client := &http.Client{
		Transport: transport,
	}

	// 发送请求
	resp, err := client.Get(targetURL)
	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// 输出响应状态
	fmt.Printf("Status: %s\n", resp.Status)
	if strings.HasPrefix(resp.Status, "3") {
		fmt.Printf("Location: %s\n", resp.Header.Get("Location"))
	}
	fmt.Printf("\n")

	// 输出响应内容
	io.Copy(os.Stdout, resp.Body)
}
