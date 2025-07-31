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
	"github.com/spf13/viper"
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
	cobra.OnInitialize(initConfig)
	defaultConfig := core.DefaultSuo5Config()

	rootCmd.Flags().StringP("config", "c", "", "the filepath for config file (json, yaml, toml)")
	rootCmd.Flags().StringP("target", "t", defaultConfig.Target, "the remote server url, ex: http://localhost:8080/suo5.jsp")
	rootCmd.Flags().StringP("listen", "l", defaultConfig.Listen, "listen address of socks5 server")
	rootCmd.Flags().StringP("method", "m", defaultConfig.Method, "http request method")
	rootCmd.Flags().StringP("redirect", "r", defaultConfig.RedirectURL, "redirect to the url if host not matched, used to bypass load balance")
	rootCmd.Flags().Bool("no-auth", defaultConfig.NoAuth, "disable socks5 authentication")
	rootCmd.Flags().String("auth", "", "socks5 creds, username:password, leave empty to auto generate")
	rootCmd.Flags().String("mode", string(defaultConfig.Mode), "connection mode, choices are auto, full, half")
	uaHeader := ""
	for _, h := range defaultConfig.RawHeader {
		if strings.HasPrefix(strings.ToLower(h), "user-agent:") {
			uaHeader = strings.TrimSpace(strings.SplitN(h, ":", 2)[1])
			break
		}
	}
	rootCmd.Flags().String("ua", uaHeader, "set the request User-Agent")
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

	_ = viper.BindPFlag("target", rootCmd.Flags().Lookup("target"))
	_ = viper.BindPFlag("listen", rootCmd.Flags().Lookup("listen"))
	_ = viper.BindPFlag("method", rootCmd.Flags().Lookup("method"))
	_ = viper.BindPFlag("redirect_url", rootCmd.Flags().Lookup("redirect"))
	_ = viper.BindPFlag("no_auth", rootCmd.Flags().Lookup("no-auth"))
	_ = viper.BindPFlag("auth", rootCmd.Flags().Lookup("auth"))
	_ = viper.BindPFlag("mode", rootCmd.Flags().Lookup("mode"))
	_ = viper.BindPFlag("ua", rootCmd.Flags().Lookup("ua"))
	_ = viper.BindPFlag("raw_header", rootCmd.Flags().Lookup("header"))
	_ = viper.BindPFlag("timeout", rootCmd.Flags().Lookup("timeout"))
	_ = viper.BindPFlag("buffer_size", rootCmd.Flags().Lookup("buf-size"))
	_ = viper.BindPFlag("upstream_proxy", rootCmd.Flags().Lookup("proxy"))
	_ = viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))
	_ = viper.BindPFlag("disable_heartbeat", rootCmd.Flags().Lookup("no-heartbeat"))
	_ = viper.BindPFlag("disable_gzip", rootCmd.Flags().Lookup("no-gzip"))
	_ = viper.BindPFlag("enable_cookiejar", rootCmd.Flags().Lookup("jar"))
	_ = viper.BindPFlag("test_exit", rootCmd.Flags().Lookup("test-exit"))
	_ = viper.BindPFlag("exclude_domain", rootCmd.Flags().Lookup("exclude-domain"))
	_ = viper.BindPFlag("exclude_domain_file", rootCmd.Flags().Lookup("exclude-domain-file"))
	_ = viper.BindPFlag("forward_target", rootCmd.Flags().Lookup("forward"))

}

func initConfig() {
	// This function will be called by cobra.OnInitialize
}

func run(cmd *cobra.Command, args []string) error {
	// Start with a config object populated with default values
	cfg := core.DefaultSuo5Config()

	// Load config from file, overwriting defaults
	configPath, _ := cmd.Flags().GetString("config")
	if err := config.LoadConfig(configPath, cfg); err != nil {
		return err
	}

	// Viper has already prioritized command-line flags over config file values.
	// Now, we just need to handle a few special cases and validations.

	// Handle the 'auth' string to set username and password.
	if viper.GetString("auth") != "" {
		auth := viper.GetString("auth")
		parts := strings.Split(auth, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid socks credentials, expected username:password")
		}
		cfg.Username = parts[0]
		cfg.Password = parts[1]
		cfg.NoAuth = false
	} else if cfg.Username == "" && !cfg.NoAuth {
		cfg.Username = "suo5"
		cfg.Password = core.RandString(8)
	}

	// Handle User-Agent from 'ua' flag, adding it to RawHeader.
	if viper.IsSet("ua") {
		ua := viper.GetString("ua")
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

	// Handle exclude-domain-file
	if viper.GetString("exclude_domain_file") != "" {
		excludeFile := viper.GetString("exclude_domain_file")
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

	if cfg.Target == "" {
		return fmt.Errorf("target is required, please specify it via -t flag or in the config file")
	}

	if cfg.Debug {
		log.Default.SetLevel("debug")
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
