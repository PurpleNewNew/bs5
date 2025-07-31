package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/PurpleNewNew/bs5/pkg/config"
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

	rootCmd.Flags().StringP("config", "c", "", "the filepath for config file (json, yaml, toml)")
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
}

func run(cmd *cobra.Command, args []string) error {
	// Start with a config object populated with default values
	cfg := core.DefaultSuo5Config()

	// Load config from file, overwriting defaults
	configPath, _ := cmd.Flags().GetString("config")
	if err := config.LoadConfig(configPath, cfg); err != nil {
		return err
	}

	// Override config with command line flags
	if cmd.Flags().Changed("target") {
		cfg.Target, _ = cmd.Flags().GetString("target")
	}
	if cmd.Flags().Changed("listen") {
		cfg.Listen, _ = cmd.Flags().GetString("listen")
	}
	if cmd.Flags().Changed("method") {
		cfg.Method, _ = cmd.Flags().GetString("method")
	}
	if cmd.Flags().Changed("redirect") {
		cfg.RedirectURL, _ = cmd.Flags().GetString("redirect")
	}
	if cmd.Flags().Changed("no-auth") {
		cfg.NoAuth, _ = cmd.Flags().GetBool("no-auth")
	}
	if cmd.Flags().Changed("auth") {
		auth, _ := cmd.Flags().GetString("auth")
		if auth != "" {
			parts := strings.Split(auth, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid socks credentials, expected username:password")
			}
			cfg.Username = parts[0]
			cfg.Password = parts[1]
			cfg.NoAuth = false
		}
	}
	if cmd.Flags().Changed("mode") {
		modeStr, _ := cmd.Flags().GetString("mode")
		cfg.Mode = core.ConnectionType(modeStr)
	}
	if cmd.Flags().Changed("ua") {
		ua, _ := cmd.Flags().GetString("ua")
		// Find and replace User-Agent header
		found := false
		for i, h := range cfg.RawHeader {
			if strings.HasPrefix(strings.ToLower(h), "user-agent:") {
				cfg.RawHeader[i] = "User-Agent: " + ua
				found = true
				break
			}
		}
		if !found {
			cfg.RawHeader = append(cfg.RawHeader, "User-Agent: "+ua)
		}
	}
	if cmd.Flags().Changed("header") {
		headers, _ := cmd.Flags().GetStringSlice("header")
		cfg.RawHeader = append(cfg.RawHeader, headers...)
	}
	if cmd.Flags().Changed("timeout") {
		cfg.Timeout, _ = cmd.Flags().GetInt("timeout")
	}
	if cmd.Flags().Changed("buf-size") {
		cfg.BufferSize, _ = cmd.Flags().GetInt("buf-size")
	}
	if cmd.Flags().Changed("proxy") {
		cfg.UpstreamProxy, _ = cmd.Flags().GetStringSlice("proxy")
	}
	if cmd.Flags().Changed("debug") {
		cfg.Debug, _ = cmd.Flags().GetBool("debug")
	}
	if cmd.Flags().Changed("no-heartbeat") {
		cfg.DisableHeartbeat, _ = cmd.Flags().GetBool("no-heartbeat")
	}
	if cmd.Flags().Changed("no-gzip") {
		cfg.DisableGzip, _ = cmd.Flags().GetBool("no-gzip")
	}
	if cmd.Flags().Changed("jar") {
		cfg.EnableCookieJar, _ = cmd.Flags().GetBool("jar")
	}
	if cmd.Flags().Changed("test-exit") {
		cfg.TestExit, _ = cmd.Flags().GetString("test-exit")
	}
	if cmd.Flags().Changed("exclude-domain") {
		exclude, _ := cmd.Flags().GetStringSlice("exclude-domain")
		cfg.ExcludeDomain = append(cfg.ExcludeDomain, exclude...)
	}
	if cmd.Flags().Changed("exclude-domain-file") {
		excludeFile, _ := cmd.Flags().GetString("exclude-domain-file")
		if excludeFile != "" {
			data, err := os.ReadFile(excludeFile)
			if err != nil {
				return err
			}
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line != "" {
					cfg.ExcludeDomain = append(cfg.ExcludeDomain, line)
				}
			}
		}
	}
	if cmd.Flags().Changed("forward") {
		cfg.ForwardTarget, _ = cmd.Flags().GetString("forward")
	}

	if cfg.Debug {
		log.Default.SetLevel("debug")
	}

	if cfg.Username == "" && !cfg.NoAuth {
		cfg.Username = "suo5"
		cfg.Password = core.RandString(8)
	}

	if !(cfg.Mode == core.AutoDuplex || cfg.Mode == core.FullDuplex || cfg.Mode == core.HalfDuplex) {
		return fmt.Errorf("invalid mode, expected auto or full or half")
	}

	if cfg.BufferSize < 512 || cfg.BufferSize > 1024000 {
		return fmt.Errorf("inproper buffer size, 512~1024000")
	}

	ctx, cancel := signalCtx()
	defer cancel()
	return ctrl.Run(ctx, cfg)
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
