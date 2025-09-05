package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
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
			var configFileNotFoundError viper.ConfigFileNotFoundError
			if !errors.As(err, &configFileNotFoundError) {
				return fmt.Errorf("error reading config file: %w", err)
			}
		}
	}

	return nil
}
