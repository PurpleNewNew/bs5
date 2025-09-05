package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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

var Version = "v0.0.2" // This will be overwritten by -ldflags

var rootCmd = &cobra.Command{
	Use:     "bs5",
	Short:   "A high-performance http tunnel",
	Version: Version,
	RunE:    run,
}

func main() {
	log.Default.SetTimeFormat("01-02 15:04")
	log.Infof("bs5 version %s", Version) // Print version on start
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	defaultConfig := core.DefaultSuo5Config()

	// Define flags
	rootCmd.Flags().StringP("config", "c", "", "the filepath for config file (json, yaml, toml)")
	rootCmd.Flags().StringP("target", "t", "", "the remote server url, ex: http://localhost:8080/suo5.jsp")
	rootCmd.Flags().StringP("listen", "l", defaultConfig.Listen, "listen address of socks5 server")
	rootCmd.Flags().StringP("method", "m", defaultConfig.Method, "http request method")
	rootCmd.Flags().StringP("redirect", "r", defaultConfig.RedirectURL, "redirect to the url if host not matched, used to bypass load balance")
	rootCmd.Flags().Bool("no-auth", defaultConfig.NoAuth, "disable socks5 authentication")
	rootCmd.Flags().String("auth", "", "socks5 creds, username:password, leave empty to auto generate")
	rootCmd.Flags().String("mode", string(defaultConfig.Mode), "connection mode, choices are auto, full, half")
	rootCmd.Flags().String("ua", "", "set the request User-Agent")
	rootCmd.Flags().StringSliceP("header", "H", nil, "use extra header, ex -H 'Cookie: abc'")
	rootCmd.Flags().Int("timeout", defaultConfig.Timeout, "request timeout in seconds")
	rootCmd.Flags().Int("buf-size", defaultConfig.BufferSize, "request max body size")
	rootCmd.Flags().StringSliceP("proxy", "p", nil, "set upstream proxy, support socks5/http(s), eg: socks5://127.0.0.1:7890")
	rootCmd.Flags().BoolP("debug", "d", defaultConfig.Debug, "debug the traffic, print more details")
	rootCmd.Flags().Bool("no-heartbeat", defaultConfig.DisableHeartbeat, "disable heartbeat to the remote server which will send data every 5s")
	rootCmd.Flags().Bool("no-gzip", defaultConfig.DisableGzip, "disable gzip compression, which will improve compatibility with some old servers")
	rootCmd.Flags().BoolP("jar", "j", defaultConfig.EnableCookieJar, "enable cookiejar")
	rootCmd.Flags().StringP("test-exit", "T", "", "test a real connection, if success exit(0), else exit(1)")
	rootCmd.Flags().StringSliceP("exclude-domain", "E", nil, "exclude certain domain name for proxy, ex -E 'portswigger.net'")
	rootCmd.Flags().String("exclude-domain-file", "", "exclude certain domains for proxy in a file, one domain per line")
	rootCmd.Flags().StringP("forward", "f", defaultConfig.ForwardTarget, "forward target address, enable forward mode when specified")
}

func initConfig() {
	// Get config path from flag
	configPath, _ := rootCmd.Flags().GetString("config")
	if err := config.InitConfig(configPath); err != nil {
		log.Fatal(err)
	}

	// Bind flags to viper. This is the correct way to ensure precedence.
	bindFlag := func(key, flagName string) {
		if err := viper.BindPFlag(key, rootCmd.Flags().Lookup(flagName)); err != nil {
			log.Fatalf("Failed to bind %s flag: %v", flagName, err)
		}
	}

	bindFlag("target", "target")
	bindFlag("listen", "listen")
	bindFlag("method", "method")
	bindFlag("redirect_url", "redirect")
	bindFlag("no_auth", "no-auth")
	bindFlag("auth", "auth")
	bindFlag("mode", "mode")
	bindFlag("ua", "ua")
	bindFlag("raw_header", "header")
	bindFlag("timeout", "timeout")
	bindFlag("buffer_size", "buf-size")
	bindFlag("upstream_proxy", "proxy")
	bindFlag("debug", "debug")
	bindFlag("disable_heartbeat", "no-heartbeat")
	bindFlag("disable_gzip", "no-gzip")
	bindFlag("enable_cookiejar", "jar")
	bindFlag("test_exit", "test-exit")
	bindFlag("exclude_domain", "exclude-domain")
	bindFlag("exclude_domain_file", "exclude-domain-file")
	bindFlag("forward_target", "forward")
}

func run(_ *cobra.Command, _ []string) error {
	// Start with default values and unmarshal all configuration sources
	cfg := core.DefaultSuo5Config()
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// If in test-and-exit mode, the target for the connection check
	// should be the URL specified by the -T flag itself. This overrides
	// any target from the config file.
	if testExitURL := viper.GetString("test_exit"); testExitURL != "" {
		cfg.Target = testExitURL
	}

	// --- Configuration Validation and Finalization ---

	// Handle the 'auth' string to set username and password.
	if viper.GetString("auth") != "" {
		auth := viper.GetString("auth")
		parts := strings.Split(auth, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid socks credentials, expected non-empty username:password")
		}
		cfg.Username = parts[0]
		cfg.Password = parts[1]
		cfg.NoAuth = false // Explicit auth overrides no-auth
	} else if cfg.Username == "" && !cfg.NoAuth {
		// Auto-generate credentials if not provided
		cfg.Username = "suo5"
		cfg.Password = core.RandString(8)
	}

	// Handle User-Agent from 'ua' flag, adding/overwriting it in RawHeader.
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
			return fmt.Errorf("failed to read exclude-domain-file: %w", err)
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				cfg.ExcludeDomain = append(cfg.ExcludeDomain, line)
			}
		}
	}

	// --- Final Validation Checks ---

	if cfg.Target == "" {
		return fmt.Errorf("target is required, please specify it via -t flag or in the config file")
	}

	// Validate HTTP method
	method := strings.ToUpper(cfg.Method)
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodHead, http.MethodOptions:
		cfg.Method = method // Ensure it's uppercase
	default:
		return fmt.Errorf("invalid http method: %s", cfg.Method)
	}

	if cfg.Debug {
		log.Default.SetLevel("debug")
	}

	if !(cfg.Mode == core.AutoDuplex || cfg.Mode == core.FullDuplex || cfg.Mode == core.HalfDuplex) {
		return fmt.Errorf("invalid mode, expected auto, full, or half")
	}

	if cfg.BufferSize < 512 || cfg.BufferSize > 1024000 {
		return fmt.Errorf("buffer size must be between 512 and 1024000 bytes")
	}

	// Validate test-exit URL if provided
	if testExitURL := viper.GetString("test_exit"); testExitURL != "" {
		if _, err := url.Parse(testExitURL); err != nil {
			return fmt.Errorf("invalid test-exit URL: %w", err)
		}
	}

	// Validate forward target if provided
	if cfg.ForwardTarget != "" {
		if !strings.Contains(cfg.ForwardTarget, "://") && !strings.Contains(cfg.ForwardTarget, ":") {
			return fmt.Errorf("forward target must be in format host:port or a full URL")
		}
	}

	// Validate auth conflicts
	if cfg.NoAuth && viper.GetString("auth") != "" {
		return fmt.Errorf("--no-auth and --auth flags are mutually exclusive")
	}

	// --- Run Application ---

	ctx, cancel := signalCtx()
	defer cancel()
	log.Infof("Starting controller...")
	return ctrl.Run(ctx, cfg)
}

func signalCtx() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		<-ch
		log.Info("Interrupt signal received, shutting down...")
		cancel()
	}()
	return ctx, cancel
}
