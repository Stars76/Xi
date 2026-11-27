//go:build windows

package uiapp

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	autostartRegPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartValueName = "Xi"
)

// legacyAutostartValueNames are the registry values earlier names of the app registered.
var legacyAutostartValueNames = []string{"526Hz"}

func AutostartSupported() bool { return true }

func formatAutostartCommand(exePath string) string {
	return fmt.Sprintf("\"%s\" --start-hidden", strings.TrimSpace(exePath))
}

func IsAutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartRegPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	for _, name := range append([]string{autostartValueName}, legacyAutostartValueNames...) {
		if val, _, err := k.GetStringValue(name); err == nil && strings.TrimSpace(val) != "" {
			return true
		}
	}
	return false
}

func SetAutostartEnabled(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, autostartRegPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	// An entry left by an earlier name would start a second, stale copy: always drop it.
	for _, name := range legacyAutostartValueNames {
		_ = k.DeleteValue(name)
	}
	if !enable {
		err = k.DeleteValue(autostartValueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(autostartValueName, formatAutostartCommand(exe))
}
