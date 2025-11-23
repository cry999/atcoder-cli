package config

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/cry999/atcoder-cli/contests/adt"
)

// Config represents the configuration for the CLI tool.
type Config struct {
	WorkDir string     `toml:"workdir"`
	ADT     adt.Config `toml:"adt"`
}

// LoadConfig loads the configuration from the specified file path.
func LoadConfig(ctx context.Context) (*Config, error) {
	// load config
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		var err error
		configHome, err = os.UserConfigDir()
		if err != nil {
			slog.ErrorContext(ctx, "failed to get user config dir", slog.String("err", err.Error()))
			return nil, err
		}
	}
	configFilePath := filepath.Join(configHome, "atcoder-cli", "config.toml")
	configFile, err := os.Open(configFilePath)
	if err != nil {
		slog.ErrorContext(ctx, "failed to open config file", slog.String("file", configFilePath), slog.String("err", err.Error()))
		return nil, err
	}
	defer configFile.Close()

	var config Config
	if _, err := toml.NewDecoder(configFile).Decode(&config); err != nil {
		slog.ErrorContext(ctx, "failed to load config file", slog.String("file", configFilePath), slog.String("err", err.Error()))
		return nil, err
	}

	config.WorkDir = os.ExpandEnv(config.WorkDir)
	if config.WorkDir == "" {
		config.WorkDir, err = os.Getwd()
		if err != nil {
			slog.ErrorContext(ctx, "failed to get current working directory", slog.String("err", err.Error()))
			return nil, err
		}
	}
	if config.ADT.DefaultLevel == "" {
		config.ADT.DefaultLevel = adt.LevelAll
	}

	return &config, nil
}

// Dump writes the configuration to the provided writer in TOML format.
func (c *Config) Dump(w io.Writer) error {
	enc := toml.NewEncoder(os.Stdout)
	enc.Indent = "  "
	return enc.Encode(c)
}
