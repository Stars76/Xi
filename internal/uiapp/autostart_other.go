//go:build !windows

package uiapp

import "errors"

func AutostartSupported() bool { return false }

func IsAutostartEnabled() bool { return false }

func SetAutostartEnabled(bool) error {
	return errors.New("autostart is only supported on Windows")
}
