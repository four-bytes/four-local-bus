package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// PortInfo is written to the port discovery file.
type PortInfo struct {
	Port int `json:"port"`
}

// WritePortFile writes the port number to the discovery file.
func WritePortFile(port int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".cache", "opencode", "plugin-bus")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(PortInfo{Port: port})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "port.json"), data, 0644)
}

// CleanupPortFile removes the port discovery file.
func CleanupPortFile() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(home, ".cache", "opencode", "plugin-bus", "port.json"))
}
