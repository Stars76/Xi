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
	autostartValueName = "526Hz"
)

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
	val, _, err := k.GetStringValue(autostartValueName)
	return err == nil && strings.TrimSpace(val) != ""
}

func SetAutostartEnabled(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, autostartRegPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
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
