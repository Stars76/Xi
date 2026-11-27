package uiapp

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/Stars76/526Hz/internal/assets"
	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo"
)

type TrayController struct {
	mu          sync.Mutex
	app         *App
	tray        *mygo.Tray
	lastTitle   string
	lastMenuSig string
	rebuild     *time.Timer
}

func NewTray(app *App) *TrayController {
	return &TrayController{app: app}
}

func (tc *TrayController) Exists() bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.tray != nil
}

func (tc *TrayController) Create() {
	tc.mu.Lock()
	if tc.tray != nil {
		tc.mu.Unlock()
		return
	}
	isMac := runtime.GOOS == "darwin"
	iconBytes := assets.AppIconPNG
	if isMac {
		iconBytes = assets.TrayTemplatePNG
	}
	tr, err := mygo.NewTray(mygo.TrayOptions{
		ID:              "526hz-tray",
		Icon:            iconBytes,
		IconIsTemplate:  isMac,
		MonospacedTitle: isMac,
		ToolTip:         "526Hz",
	})
	if err != nil {
		tc.mu.Unlock()
		tc.app.Log.Warn("托盘创建失败", err)
		return
	}
	tc.tray = tr
	tc.mu.Unlock()

	if isMac {
		tr.OnClick(func() { tc.app.OpenMainWindow() })
		tr.OnRightClick(func() {
			tr.SetMenu(tc.buildMenu(tc.app.Timer.Snapshot()))
			defer tr.SetMenu(nil)
			tr.PopUpMenu()
		})
	} else {
		tr.SetMenu(tc.buildMenu(tc.app.Timer.Snapshot()))
		tr.OnClick(func() { tc.app.OpenMainWindow() })
	}
	tc.Refresh(tc.app.Timer.Snapshot())
	tc.app.Log.Info("托盘已就绪")
}

func (tc *TrayController) Destroy() {
	tc.mu.Lock()
	if tc.rebuild != nil {
		tc.rebuild.Stop()
		tc.rebuild = nil
	}
	tr := tc.tray
	tc.tray = nil
	tc.mu.Unlock()
	if tr != nil {
		tr.Destroy()
	}
}

func trayMenuSignature(snap timer.Snapshot) string {
	t := snap.Timer
	taskTitle := ""
	if t.Task != nil {
		taskTitle = t.Task.Title
	}
	planKey, planIdx := "", -1
	if t.Plan != nil {
		planKey = t.Plan.Key
		planIdx = t.Plan.Index
	}
	return fmt.Sprintf("%s|%d|%v|%v|%v|%s|%s|%d",
		t.Phase, t.TotalSec, t.Running, t.Paused, t.Idle, taskTitle, planKey, planIdx)
}

func (tc *TrayController) Refresh(snap timer.Snapshot) {
	tc.mu.Lock()
	tr := tc.tray
	if tr == nil {
		tc.mu.Unlock()
		return
	}
	ttl := trayTitle(snap)
	titleChanged := ttl != tc.lastTitle
	if titleChanged {
		tc.lastTitle = ttl
	}
	menuSig := trayMenuSignature(snap)
	menuChanged := menuSig != tc.lastMenuSig
	if menuChanged {
		tc.lastMenuSig = menuSig
	}
	tc.mu.Unlock()

	if titleChanged {
		tr.SetToolTip("526Hz · " + ttl)
		if runtime.GOOS == "darwin" {
			t := snap.Timer
			if t.Running || t.Paused {
				tr.SetTitle(dateutil.FmtClock(t.LeftSec))
			} else {
				tr.SetTitle("")
			}
		}
	}
	if runtime.GOOS == "darwin" || !menuChanged {
		return
	}

	tc.mu.Lock()
	if tc.rebuild == nil {
		tc.rebuild = time.AfterFunc(250*time.Millisecond, func() {
			tc.mu.Lock()
			tc.rebuild = nil
			cur := tc.tray
			tc.mu.Unlock()
			if cur != nil {
				cur.SetMenu(tc.buildMenu(tc.app.Timer.Snapshot()))
			}
		})
	}
	tc.mu.Unlock()
}

func trayTitle(snap timer.Snapshot) string {
	t := snap.Timer
	if t.Running {
		return "专注中 " + dateutil.FmtClock(t.LeftSec)
	}
	if t.Paused {
		return "已暂停 " + dateutil.FmtClock(t.LeftSec)
	}
	return t.PhaseLabel + " · 待开始"
}

func (tc *TrayController) buildMenu(snap timer.Snapshot) *mygo.Menu {
	t := snap.Timer
	toggleLabel := "▶ 开始"
	if t.Running {
		toggleLabel = "⏸ 暂停"
	}
	mark := func(active bool) string {
		if active {
			return "● "
		}
		return "○ "
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			Label:    fmt.Sprintf("%s（今日 %d 分钟 / %d 轮）", trayTitle(snap), snap.Stats.Minutes, snap.Stats.Rounds),
			Disabled: true,
		},
		mygo.Separator(),
		{
			Label: toggleLabel,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				if tc.app.Timer.Snapshot().Timer.Running {
					tc.app.Timer.Command("pause")
				} else {
					tc.app.Timer.Command("start")
				}
			},
		},
		{
			Label: "↺ 重置本轮",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.Timer.Command("reset")
			},
		},
		mygo.Separator(),
		{
			Label: fmt.Sprintf("%s专注 %d 分钟", mark(t.Phase == "focus"), (snap.Phases.Focus+30)/60),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.Timer.Command("setPhase", timer.CmdPayload{Phase: "focus"})
			},
		},
		{
			Label: fmt.Sprintf("%s短休息 %d 分钟", mark(t.Phase == "short"), (snap.Phases.Short+30)/60),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.Timer.Command("setPhase", timer.CmdPayload{Phase: "short"})
			},
		},
		{
			Label: fmt.Sprintf("%s长休息 %d 分钟", mark(t.Phase == "long"), (snap.Phases.Long+30)/60),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.Timer.Command("setPhase", timer.CmdPayload{Phase: "long"})
			},
		},
		mygo.Separator(),
		{
			Label: "⏱ 打开专注钟小窗",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.OpenTimerWindow()
			},
		},
		{
			Label: "🧠 显示主窗口",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.OpenMainWindow()
			},
		},
		mygo.Separator(),
		{
			Label: "退出 526Hz",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				tc.app.quitting = true
				mygo.App.Quit()
			},
		},
	})
}
