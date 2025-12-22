package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gopro-gui/metadata"
)

// Config holds persistent application settings
type Config struct {
	LastWorkingDir string            `json:"last_working_dir"`
	LastOutputDir  string            `json:"last_output_dir"`
	Periods        []metadata.Period `json:"periods"`
	SecondsBefore  float64           `json:"seconds_before"`
	SecondsAfter   float64           `json:"seconds_after"`
}

// DefaultConfig returns a new config with default values
func DefaultConfig() *Config {
	return &Config{
		SecondsBefore: 8.0,
		SecondsAfter:  2.0,
	}
}

// configPath returns the path to the config file
func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "gopro-clip-extractor")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(appDir, "config.json"), nil
}

// Load loads the config from disk, returning defaults if not found
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}
	defer file.Close()

	cfg := DefaultConfig()
	if err := json.NewDecoder(file).Decode(cfg); err != nil {
		return DefaultConfig(), nil
	}

	return cfg, nil
}

// Save saves the config to disk
func (c *Config) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c)
}
