package config

import (
	"fmt"
	"github.com/PurpleNewNew/bs5/pkg/core"
	"github.com/spf13/viper"
	"strings"
)

// LoadConfig loads configuration from file and environment variables,
// and unmarshals it into the provided cfg object, overwriting the defaults.
func LoadConfig(configPath string, cfg *core.Suo5Config) error {
	v := viper.New()

	// Load from config file if provided
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// Search for config file in standard locations
		v.SetConfigName("config")
		v.AddConfigPath("assets/config") // Look in the assets/config directory
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.bs5")
		v.AddConfigPath("/etc/bs5")
		// It's okay if the config file is not found, but not other errors
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	// Load from environment variables
	v.SetEnvPrefix("BS5")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal the config into our struct, overwriting defaults
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}
