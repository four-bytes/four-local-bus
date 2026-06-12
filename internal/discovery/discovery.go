package discovery

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// PortInfo is written to the port discovery file on startup.
type PortInfo struct {
	Port int `json:"port"`
}

// portFilePath returns the canonical path to the port discovery file.
func portFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".cache", "opencode", "plugin-bus", "port.json"), nil
}

// WritePortFile writes the port number to the discovery file
// at ~/.cache/opencode/plugin-bus/port.json.
func WritePortFile(port int) error {
	path, err := portFilePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create cache directory: %w", err)
	}
	data, err := json.Marshal(PortInfo{Port: port})
	if err != nil {
		return fmt.Errorf("cannot marshal port info: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("cannot write port file: %w", err)
	}
	return nil
}

// ReadPortFile reads the port number from the discovery file.
// Returns an error if the file does not exist or cannot be parsed.
func ReadPortFile() (int, error) {
	path, err := portFilePath()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("cannot read port file: %w", err)
	}
	var info PortInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return 0, fmt.Errorf("cannot parse port file: %w", err)
	}
	return info.Port, nil
}

// CheckHealth performs a GET /health request against the given port
// on localhost to determine if a bus instance is running.
func CheckHealth(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// CleanupPortFile removes the port discovery file.
// Returns nil if the file does not exist (idempotent).
func CleanupPortFile() error {
	path, err := portFilePath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
