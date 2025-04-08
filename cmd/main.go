package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/PurpleNewNew/bs5/pkg/core"
	"github.com/PurpleNewNew/bs5/pkg/ctrl"
	log "github.com/kataras/golog"
	"github.com/spf13/cobra"
)

var Version = "v0.0.0"

func main() {
	log.Default.SetTimeFormat("01-02 15:04")
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:     "suo5",
	Short:   "A high-performance http tunnel",
	Version: Version,
	RunE:    run,
}

func init() {
	defaultConfig := core.DefaultSuo5Config()

	rootCmd.Flags().StringP("config", "c", "", "the filepath for json config file")
	rootCmd.Flags().StringP("target", "t", defaultConfig.Target, "the remote server url, ex: http://localhost:8080/suo5.jsp")
	rootCmd.Flags().StringP("listen", "l", defaultConfig.Listen, "listen address of socks5 server")
	rootCmd.Flags().StringP("method", "m", defaultConfig.Method, "http request method")
	rootCmd.Flags().StringP("redirect", "r", defaultConfig.RedirectURL, "redirect to the url if host not matched, used to bypass load balance")
	rootCmd.Flags().Bool("no-auth", defaultConfig.NoAuth, "disable socks5 authentication")
	rootCmd.Flags().String("auth", "", "socks5 creds, username:password, leave empty to auto generate")
	rootCmd.Flags().String("mode", string(defaultConfig.Mode), "connection mode, choices are auto, full, half")
	rootCmd.Flags().String("ua", "Mozilla/5.0 (Linux; Android 6.0; Nexus 5 Build/MRA58N) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.1.2.3", "set the request User-Agent")
	rootCmd.Flags().StringSliceP("header", "H", nil, "use extra header, ex -H 'Cookie: abc'")
	rootCmd.Flags().Int("timeout", defaultConfig.Timeout, "request timeout in seconds")
	rootCmd.Flags().Int("buf-size", defaultConfig.BufferSize, "request max body size")
	rootCmd.Flags().StringSliceP("proxy", "p", defaultConfig.UpstreamProxy, "set upstream proxy, support socks5/http(s), eg: socks5://127.0.0.1:7890")
	rootCmd.Flags().BoolP("debug", "d", defaultConfig.Debug, "debug the traffic, print more details")
	rootCmd.Flags().Bool("no-heartbeat", defaultConfig.DisableHeartbeat, "disable heartbeat to the remote server which will send data every 5s")
	rootCmd.Flags().Bool("no-gzip", defaultConfig.DisableGzip, "disable gzip compression, which will improve compatibility with some old servers")
	rootCmd.Flags().BoolP("jar", "j", defaultConfig.EnableCookieJar, "enable cookiejar")
	rootCmd.Flags().StringP("test-exit", "T", "", "test a real connection, if success exit(0), else exit(1)")
	rootCmd.Flags().StringSliceP("exclude-domain", "E", nil, "exclude certain domain name for proxy, ex -E 'portswigger.net'")
	rootCmd.Flags().String("exclude-domain-file", "", "exclude certain domains for proxy in a file, one domain per line")
	rootCmd.Flags().StringP("forward", "f", defaultConfig.ForwardTarget, "forward target address, enable forward mode when specified")

	rootCmd.MarkFlagRequired("target")
}

func run(cmd *cobra.Command, args []string) error {
	if debug, _ := cmd.Flags().GetBool("debug"); debug {
		log.Default.SetLevel("debug")
	}

	listen, _ := cmd.Flags().GetString("listen")
	target, _ := cmd.Flags().GetString("target")
	noAuth, _ := cmd.Flags().GetBool("no-auth")
	auth, _ := cmd.Flags().GetString("auth")
	modeStr, _ := cmd.Flags().GetString("mode")
	mode := core.ConnectionType(modeStr)
	ua, _ := cmd.Flags().GetString("ua")
	bufSize, _ := cmd.Flags().GetInt("buf-size")
	timeout, _ := cmd.Flags().GetInt("timeout")
	debug, _ := cmd.Flags().GetBool("debug")
	proxy, _ := cmd.Flags().GetStringSlice("proxy")
	method, _ := cmd.Flags().GetString("method")
	redirect, _ := cmd.Flags().GetString("redirect")
	header, _ := cmd.Flags().GetStringSlice("header")
	noHeartbeat, _ := cmd.Flags().GetBool("no-heartbeat")
	noGzip, _ := cmd.Flags().GetBool("no-gzip")
	jar, _ := cmd.Flags().GetBool("jar")
	testExit, _ := cmd.Flags().GetString("test-exit")
	exclude, _ := cmd.Flags().GetStringSlice("exclude-domain")
	excludeFile, _ := cmd.Flags().GetString("exclude-domain-file")
	forward, _ := cmd.Flags().GetString("forward")
	configFile, _ := cmd.Flags().GetString("config")

	var username, password string
	if auth == "" {
		if !noAuth {
			username = "suo5"
			password = core.RandString(8)
		}
	} else {
		parts := strings.Split(auth, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid socks credentials, expected username:password")
		}
		username = parts[0]
		password = parts[1]
		noAuth = false
	}
	if !(mode == core.AutoDuplex || mode == core.FullDuplex || mode == core.HalfDuplex) {
		return fmt.Errorf("invalid mode, expected auto or full or half")
	}

	if bufSize < 512 || bufSize > 1024000 {
		return fmt.Errorf("inproper buffer size, 512~1024000")
	}
	header = append(header, "User-Agent: "+ua)

	if excludeFile != "" {
		data, err := os.ReadFile(excludeFile)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				exclude = append(exclude, line)
			}
		}
	}

	config := &core.Suo5Config{
		Listen:           listen,
		Target:           target,
		NoAuth:           noAuth,
		Username:         username,
		Password:         password,
		Mode:             mode,
		BufferSize:       bufSize,
		Timeout:          timeout,
		Debug:            debug,
		UpstreamProxy:    proxy,
		Method:           method,
		RedirectURL:      redirect,
		RawHeader:        header,
		DisableHeartbeat: noHeartbeat,
		DisableGzip:      noGzip,
		EnableCookieJar:  jar,
		TestExit:         testExit,
		ExcludeDomain:    exclude,
		ForwardTarget:    forward,
	}

	if configFile != "" {
		log.Infof("loading config from %s", configFile)
		data, err := os.ReadFile(configFile)
		if err != nil {
			return err
		}
		err = json.Unmarshal(data, config)
		if err != nil {
			return err
		}
	}

	ctx, cancel := signalCtx()
	defer cancel()
	return ctrl.Run(ctx, config)
}

func signalCtx() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		cancel()
	}()
	return ctx, cancel
}
