package srv

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Config represents a server configuration with basic details like name, identifier, and icon.
// It also includes the server address for connection.
type Config struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
	Icon       string `json:"icon"`
	BetaLock   bool   `json:"beta_lock"`

	Address string `json:"address"`
}

// ReadAll reads all JSON configuration files from the specified path and returns a slice of Config.
// It walks through the directory, parsing each valid JSON file.
func ReadAll(path string) ([]Config, error) {
	var configs []Config
	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(p) == ".json" {
			cfg, err := parseConfig(p)
			if err != nil {
				return fmt.Errorf("error: %w", err)
			}
			configs = append(configs, cfg)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return configs, nil
}

// parseConfig reads a file and unmarshal' its JSON data into a Config structure.
// Returns an error if reading or parsing fails.
func parseConfig(file string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(file)
	if err != nil {
		return cfg, fmt.Errorf("failed to read file %s: %w", file, err)
	}
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to parse file %s: %w", file, err)
	}
	return cfg, nil
}
