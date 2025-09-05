package config

import (
	"fmt"
	"github.com/PurpleNewNew/bs5/pkg/core"
	"github.com/spf13/viper"
	"strings"
)

// InitConfig initializes the global viper instance with config file and environment variables
// This should be called before binding flags in main.go
func InitConfig(configPath string) error {
	// Set up environment variables
	viper.SetEnvPrefix("BS5")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Load from config file if provided
	if configPath != "" {
		viper.SetConfigFile(configPath)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// Search for config file in standard locations
		viper.SetConfigName("config")
		viper.AddConfigPath("assets/config") // Look in the assets/config directory
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.bs5")
		viper.AddConfigPath("/etc/bs5")
		// It's okay if the config file is not found, but not other errors
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	return nil
}

// LoadConfig is deprecated. Use InitConfig instead and let viper handle the unmarshaling
// This is kept for backward compatibility but should be removed
func LoadConfig(configPath string, cfg *core.Suo5Config) error {
	if err := InitConfig(configPath); err != nil {
		return err
	}

	// Unmarshal the config into our struct
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}
