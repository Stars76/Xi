package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Stars76/526Hz/internal/uiapp"
	"github.com/egoist/mygo"
)

func parseArgs(args []string) (compact bool, startHidden bool, userDataDir string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--compact-timer":
			compact = true
		case "--start-hidden":
			startHidden = true
		case "--user-data":
			if i+1 < len(args) {
				userDataDir = args[i+1]
				i++
			}
		default:
			if strings.HasPrefix(args[i], "--user-data=") {
				userDataDir = strings.TrimPrefix(args[i], "--user-data=")
			}
		}
	}
	return compact, startHidden, userDataDir
}

func resolveUserDataDir(overrideData, configDir, defaultAppDir string) string {
	if overrideData != "" {
		if abs, err := filepath.Abs(overrideData); err == nil {
			return abs
		}
		return overrideData
	}
	if defaultAppDir == "" && configDir != "" {
		defaultAppDir = filepath.Join(configDir, "526Hz")
	}
	// Preserve existing user data under legacy %APPDATA%\Sisyphus unless a 526Hz store already exists.
	if configDir != "" && defaultAppDir != "" {
		newStore := filepath.Join(defaultAppDir, "sisy-store.json")
		legacyDir := filepath.Join(configDir, "Sisyphus")
		legacyStore := filepath.Join(legacyDir, "sisy-store.json")
		if _, err := os.Stat(newStore); os.IsNotExist(err) {
			if _, err := os.Stat(legacyStore); err == nil {
				return legacyDir
			}
		}
	}
	return defaultAppDir
}

func main() {
	compactOnly, startHidden, overrideData := parseArgs(os.Args[1:])

	mygo.App.SetName("526Hz")
	mygo.App.SetVersion(uiapp.AppVersion)

	cfgDir, _ := os.UserConfigDir()
	defaultDir, _ := mygo.App.Path(mygo.PathUserData)
	userDataDir := resolveUserDataDir(overrideData, cfgDir, defaultDir)
	if userDataDir != "" {
		mygo.App.SetPath(mygo.PathUserData, userDataDir)
	}

	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	application := uiapp.NewApp(uiapp.AppConfig{
		UserDataDir: userDataDir,
	})

	mygo.App.OnSecondInstance(func(args []string, _ string) {
		isCompact, _, _ := parseArgs(args)
		if isCompact {
			application.OpenTimerWindow()
		} else {
			application.OpenMainWindow()
		}
	})

	mygo.App.WhenReady(func() {
		application.StartDesktop(compactOnly, startHidden)
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
