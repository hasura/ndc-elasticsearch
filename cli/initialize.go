package cli

import (
	"os"
	"path/filepath"
)

const ConfigFileName = "configuration.json"

// initialize creates a configuration file at the specified path.
func initialize(configPath string) error {
	configFilePath := filepath.Join(configPath, ConfigFileName)
	file, err := os.Create(configFilePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}
