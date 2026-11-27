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

// appDirName is the app's ASCII name: its data directory and executable are called this.
// The name people see is 汐.
const appDirName = "Xi"

// legacyAppDirNames are the data directories of the app's earlier names, newest first.
var legacyAppDirNames = []string{"526Hz", "Sisyphus"}

func resolveUserDataDir(overrideData, configDir, defaultAppDir string) string {
	if overrideData != "" {
		if abs, err := filepath.Abs(overrideData); err == nil {
			return abs
		}
		return overrideData
	}
	if defaultAppDir == "" && configDir != "" {
		defaultAppDir = filepath.Join(configDir, appDirName)
	}
	// The app has been renamed twice (Sisyphus, then 526Hz, now 汐 / Xi). Unless a store already
	// exists under the current name, keep using the newest earlier directory that has one, so
	// nobody's data is left behind by a rename.
	if configDir != "" && defaultAppDir != "" {
		if _, err := os.Stat(filepath.Join(defaultAppDir, "sisy-store.json")); os.IsNotExist(err) {
			for _, name := range legacyAppDirNames {
				legacyDir := filepath.Join(configDir, name)
				if _, err := os.Stat(filepath.Join(legacyDir, "sisy-store.json")); err == nil {
					return legacyDir
				}
			}
		}
	}
	return defaultAppDir
}

func main() {
	compactOnly, startHidden, overrideData := parseArgs(os.Args[1:])

	mygo.App.SetName(appDirName)
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
