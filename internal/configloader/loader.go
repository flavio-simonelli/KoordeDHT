package configloader

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads a YAML file into the given struct pointer.
func LoadYAML(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("failed to parse yaml: %w", err)
	}
	return nil
}
