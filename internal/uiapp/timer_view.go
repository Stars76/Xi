package uiapp

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo/ui"
)

var (
	timerOverlayMotion = ui.ElementTransition{
		Duration:     160 * time.Millisecond,
		Ease:         ui.EaseOut,
		ExitDuration: 110 * time.Millisecond,
		Enter:        &ui.Motion{Y: -8, Opacity: 0, Scale: 0.98},
		Exit:         &ui.Motion{Y: -6, Opacity: 0, Scale: 0.98},
	}
	timerDoneCardMotion = ui.ElementTransition{
		Animation: &ui.Snappy,
		Enter:     &ui.Motion{Scale: 0.92, Opacity: 0},
		Exit:      &ui.Motion{Scale: 0.95, Opacity: 0},
	}
	countdownRoll = ui.ContentTransition{
		Numeric: true,
		Down:    true,
	}
)

func timerBarBtn(c *ui.Context, icon *ui.SVG, label, tip string, active bool, isClose bool, p Palette) ui.Element {
	b := ui.ButtonBase(c).Size(30, 30).Radius(7).Center().Label(label).Tooltip(tip).
		Cursor(ui.CursorPointer).Transition(colorFadeFast)
	hov := b.Hovered()
	if isClose && hov {
		b.Background(ui.Hex("#E81123"))
	} else if active {
		b.Background(p.AccentSoft)
	} else if hov {
		b.Background(p.HoverTint)
	}
	b.Children(func() {
		col := p.Ink2
		if isClose && hov {
			col = ui.Hex("#FFFFFF")
		} else if active {
			col = p.BlueDeep
		} else if hov {
			col = p.Navy
		}
		ui.Icon(c, icon).Size(15, 15).TextColor(col)
	})
	return b
}

func (a *App) TimerView(c *ui.Context) {
	th, p := a.paletteOf(c)
	c.SetTheme(th)
	a.consumePendingToast(c)

	if c.Shortcut(0, ui.KeyEscape) {
		if a.timerShowSet {
			a.timerShowSet = false
		} else if a.timerShowSetup {
			a.timerShowSetup = false
			a.Timer.ClearAwaitPlan()
		} else if a.timerDoneVisible {
			a.timerDoneVisible = false
		}
	}

	snap := a.Timer.Snapshot()
	t := snap.Timer
	prefs := snap.Prefs

	if a.timerMiniPill {
		a.renderTimerMiniPill(c, snap, p)
		return
	}

	if t.AwaitPlan && !a.timerShowSetup {
		a.timerShowSetup = true
	}

	remainRatio := 1.0
	if t.TotalSec > 0 {
		remainRatio = float64(t.LeftMs) / float64(int64(t.TotalSec)*1000)
	}

	// Root card: one flat paper colour behind the sea
	root := ui.Box(c).Fill().Background(p.Paper).Border(1, p.BorderSubtle).Clip()
	px, py, pIn := root.PointerPosition()
	if root.Clicked() && !a.timerShowSetup && !a.timerShowSet && !a.timerDoneVisible {
		a.handleOceanClick(px, py)
	}
	root.Children(func() {
		// Layer 0: Shared 1:1 interactive character sea canvas (synced with Focus Clock)
		sc := a.seaSceneSnapshot()
		seaBox := ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(0).PassThrough()
		a.stageSea.Draw(c, seaBox, SeaView{
			RemainRatio:  remainRatio,
			RemainSec:    t.LeftSec,
			Running:      t.Running,
			Paused:       t.Paused,
			Idle:         t.Idle,
			Motion:       prefs.Motion,
			HighContrast: prefs.HighContrast,
			Dark:         p.Dark,
			ReduceMotion: c.Preferences().ReduceMotion,
			PointerX:     px,
			PointerY:     py,
			PointerIn:    pIn && !a.timerShowSetup && !a.timerShowSet && !a.timerDoneVisible,
			Weather:      a.effectiveWeather(),
			TimeLighting: sc.TimeLighting,
			EncountersOn: sc.EncountersOn,
			TodayMinutes: snap.Stats.Minutes,
			TodayRounds:  snap.Stats.Rounds,
			Now:          a.liveNow(),
			OnEncounter:  a.recordNaturalEncounter,
		})

		// Layer 2: Main foreground column (explicit Absolute + ZIndex(2) so it ALWAYS sits cleanly ABOVE the sea & veils)
		ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(2).Children(func() {
			// Transparent draggable compact titlebar
			wbar := ui.Row(c).FillWidth().Padding(9, 10, 6, 16).Gap(3).AlignItems(ui.Center).DragWindow()
			wbar.Children(func() {
				ui.RichText(c,
					ui.Span{Text: "专注", Weight: 800, Size: 20, Color: p.Ink},
					ui.Span{Text: "钟", Weight: 800, Size: 20, Color: p.Blue},
				)
				ui.Spacer(c)

				homeBtn := timerBarBtn(c, IconList, "返回主窗", "打开主窗口", false, false, p)
				if homeBtn.Clicked() {
					a.OpenMainWindow()
				}

				pillBtn := timerBarBtn(c, IconCompact, "切换灵动条", "折叠为悬浮灵动条", false, false, p)
				if pillBtn.Clicked() {
					a.SetTimerMiniPill(true)
				}

				setBtn := timerBarBtn(c, IconSettings, "专注钟设置", "设置", a.timerShowSet, false, p)
				if setBtn.Clicked() {
					a.timerShowSet = !a.timerShowSet
					a.timerShowSetup = false
				}

				pinned := a.TimerWin != nil && a.TimerWin.IsAlwaysOnTop()
				pinBtn := timerBarBtn(c, IconPin, "切换置顶", "置顶", pinned, false, p)
				if pinBtn.Clicked() && a.TimerWin != nil {
					a.TimerWin.SetAlwaysOnTop(!pinned)
				}

				clsBtn := timerBarBtn(c, IconClose, "关闭小窗", "关闭", false, true, p)
				if clsBtn.Clicked() && a.TimerWin != nil {
					a.TimerWin.Close()
				}
			})

			// Center countdown numeral floating directly over the character sea, rolling digits
			ui.Box(c).Grow(1).FillWidth().Center().Children(func() {
				timeStr := dateutil.FmtDuration(t.LeftSec, prefs.ShowSeconds)
				numWrap := ui.Column(c).Center().Gap(6).Padding(12, 22)
				if prefs.HighContrast {
					numWrap.Radius(16).Background(p.TimerHCBack).Border(1, p.Line)
				}
				numWrap.Children(func() {
					if t.Task != nil && t.Task.Title != "" {
						ui.Row(c).Padding(3, 10).Radius(999).Background(p.TimerChipBg).Border(1, p.BorderSubtle).Children(func() {
							ui.Text(c, t.Task.Title).FontSize(11.5).FontWeight(600).TextColor(p.Navy).MaxWidth(210).SingleLine()
						})
					}
					weight := 600
					if prefs.HighContrast {
						weight = 800
					}
					ui.Text(c, timeStr).Font(FontNum).FontSize(68).FontWeight(weight).
						FontFeatures("tnum").TextColor(p.Navy).
						TextShadow(0, 0, 18, p.TimerNumGlow).
						ContentTransition(countdownRoll).SingleLine()
					if t.Plan != nil {
						ui.Textf(c, "%s · %d/%d", t.Plan.Name, t.Plan.Index+1, t.Plan.Total).
							FontSize(12).FontWeight(700).TextColor(p.BlueDeep).
							TextShadow(0, 0, 10, p.TimerNumGlow)
					}
					if snap.ClockJumpAt > 0 {
						ui.Text(c, "已校对系统时间跳变").FontSize(10.5).TextColor(p.WarnInk)
					}
				})
			})

			// Bottom dock
			a.renderTimerDock(c, snap, p)
		})

		// Layer 10: Overlays: Plan selector / Settings / Done screen
		if a.timerShowSetup {
			a.renderTimerSetupOverlay(c, snap, p, false)
		}
		if a.timerShowSet {
			a.renderTimerSettingsOverlay(c, snap, p, false)
		}
		if a.timerDoneVisible && a.timerDoneEv != nil {
			a.renderTimerDoneOverlay(c, p)
		}
	})
}

// renderTimerMiniPill renders the sleek compact 264x50 floating capsule mode.
func (a *App) renderTimerMiniPill(c *ui.Context, snap timer.Snapshot, p Palette) {
	t := snap.Timer
	prefs := snap.Prefs
	dotCol := p.IdleDot
	if t.Phase != "focus" {
		dotCol = p.Run
	} else if t.Running {
		dotCol = p.Run
	} else if t.Paused {
		dotCol = p.Warn
	}
	timeStr := dateutil.FmtDuration(t.LeftSec, prefs.ShowSeconds)
	elapsedRatio := 0.0
	if t.TotalSec > 0 && !t.Idle {
		rem := float64(t.LeftMs) / float64(int64(t.TotalSec)*1000)
		elapsedRatio = math.Max(0, math.Min(1, 1.0-rem))
	}
	pillTip := "526Hz 灵动条"
	if t.Task != nil && t.Task.Title != "" {
		pillTip = fmt.Sprintf("%s · %s", t.PhaseLabel, t.Task.Title)
	}

	bar := ui.Box(c).Fill().
		LinearGradient(ui.LinearGradient{From: p.Paper, To: p.Surface, Angle: 180, Oklab: true}).
		Border(1, p.Line).Tooltip(pillTip).DragWindow().Clip()
	bar.Children(func() {
		// Foreground compact capsule row
		ui.Row(c).Fill().Padding(0, 8, 2, 13).Gap(6).AlignItems(ui.Center).Children(func() {
			dot := ui.Box(c).Size(7.5, 7.5).Radius(999).Background(dotCol)
			if t.Running {
				pulse := dot.Loop("pill-dot-pulse", 1600*time.Millisecond, ui.EaseInOut)
				dot.Opacity(0.35 + 0.65*pulse)
			}
			ui.Text(c, timeStr).Font(FontNum).FontSize(22).FontWeight(700).FontFeatures("tnum").
				TextColor(p.Navy).ContentTransition(countdownRoll).SingleLine()

			ui.Spacer(c)

			playIcon := IconPlay
			playLabel := "开始计时"
			if t.Running {
				playIcon = IconPause
				playLabel = "暂停计时"
			}
			ppBtn := ui.ButtonBase(c).Size(30, 30).Radius(999).Center().
				LinearGradient(ui.LinearGradient{From: p.Blue, To: p.BlueDeep, Angle: 180, Oklab: true}).
				Shadow(0, 2, 8, 0, p.BtnGlow).Transition(cardFadeFast).Cursor(ui.CursorPointer).
				Label(playLabel).Tooltip(playLabel)
			ppBtn.Children(func() {
				ui.Icon(c, playIcon).Size(13, 13).TextColor(ui.Hex("#FFFFFF"))
			})
			if ppBtn.Clicked() {
				if t.Running {
					a.Timer.Command("pause")
				} else {
					a.Timer.Command("start")
				}
			}

			expBtn := timerBarBtn(c, IconExpand, "展开专注钟", "展开为字符海画中画", false, false, p).Size(28, 28)
			if expBtn.Clicked() {
				a.SetTimerMiniPill(false)
			}

			clsBtn := timerBarBtn(c, IconClose, "关闭灵动条", "关闭悬浮灵动条", false, true, p).Size(28, 28)
			if clsBtn.Clicked() && a.TimerWin != nil {
				a.TimerWin.Close()
			}
		})

		// Crisp 2.5px bottom ocean-blue progress hairline
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(2.5).ZIndex(2).PassThrough().
			Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Fill(r, p.LineSoft.Alpha(0.65), 0)
				if elapsedRatio > 0 {
					fillW := float32(math.Max(3, float64(r.W)*elapsedRatio))
					fill := ui.Rect{X: r.X, Y: r.Y, W: fillW, H: r.H}
					pt.FillGradient(fill, ui.LinearGradient{
						From:  p.Blue,
						To:    p.BlueDeep,
						Angle: 90,
					}, 0)
				}
			})
	})
}

func (a *App) renderTimerDock(c *ui.Context, snap timer.Snapshot, p Palette) {
	t := snap.Timer
	dotCol := p.IdleDot
	chipBg := p.TimerChipBg
	chipBorder := p.BorderMid
	chipInk := p.Navy
	chipText := "待机中"
	totalText := dateutil.FmtDuration(t.TotalSec, true)
	if t.Phase != "focus" {
		dotCol = p.Run
		chipBg = p.SuccessSoft
		chipBorder = p.SuccessLine
		chipInk = p.Success
		lbl := "长休息"
		if t.Phase == "short" {
			lbl = "短休息"
		}
		if !t.Running && !t.Idle {
			chipText = "休息暂停 · " + totalText
		} else {
			chipText = lbl + " · " + totalText
		}
	} else if t.Running {
		dotCol = p.Run
		if t.Recovered {
			chipBg = p.AccentSoft
			chipBorder = p.AccentVeil
			chipInk = p.BlueDeep
			chipText = "恢复中 · " + totalText
		} else {
			chipText = "专注中 · " + totalText
		}
	} else if t.Paused {
		dotCol = p.Warn
		chipBg = p.WarnBg
		chipBorder = p.WarnLine
		chipInk = p.WarnInk
		chipText = "已暂停 · " + totalText
	}

	setupText := fmt.Sprintf("%d 分钟", (t.TotalSec+30)/60)
	if t.Plan != nil {
		setupText = t.Plan.Name
	}

	ui.Column(c).FillWidth().Padding(9, 12, 11, 12).Background(p.TimerDockBg).
		BorderWidth(1, 0, 0, 0).BorderColor(p.BorderSubtle).Gap(6).Children(func() {
		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Height(32).Padding(0, 10).Radius(999).Background(chipBg).Border(1, chipBorder).
				Transition(colorFadeFast).Gap(6).AlignItems(ui.Center).Children(func() {
				dot := ui.Box(c).Size(6, 6).Radius(999).Background(dotCol)
				if t.Running {
					pulse := dot.Loop("timer-dot-pulse", 1600*time.Millisecond, ui.EaseInOut)
					dot.Opacity(0.35 + 0.65*pulse)
				}
				ui.Text(c, chipText).FontSize(12).FontWeight(700).FontFeatures("tnum").TextColor(chipInk).SingleLine()
			})
			ui.Spacer(c)

			setupBtn := ui.Button(c, setupText).Height(32).Padding(0, 10).Radius(9).FontSize(12.5).Transition(colorFadeFast)
			if setupBtn.Clicked() {
				a.timerShowSetup = !a.timerShowSetup
				a.timerShowSet = false
			}

			primaryLabel := "开始"
			if t.Running {
				primaryLabel = "暂停"
			} else if t.Paused {
				primaryLabel = "继续"
			}
			mainBtn := ui.ButtonBase(c).Height(32).Padding(0, 14).Radius(9).Center().
				LinearGradient(ui.LinearGradient{From: p.Blue, To: p.BlueDeep, Angle: 180, Oklab: true}).
				Shadow(0, 3, 10, 0, p.BtnGlow).Transition(cardFadeFast).Cursor(ui.CursorPointer).Label(primaryLabel)
			if mainBtn.Hovered() {
				mainBtn.LinearGradient(ui.LinearGradient{From: p.Blue2, To: p.Blue, Angle: 180, Oklab: true})
			}
			mainBtn.Children(func() {
				ui.Text(c, primaryLabel).FontSize(13).FontWeight(700).TextColor(ui.Hex("#FFFFFF")).SingleLine()
			})
			if mainBtn.Clicked() {
				a.timerShowSetup = false
				a.timerShowSet = false
				if t.Running {
					a.Timer.Command("pause")
				} else {
					a.Timer.Command("start")
				}
			}

			if !t.Idle {
				rstText := "重置"
				if a.resetArmed() {
					rstText = "确认放弃"
				}
				rstBtn := ui.Button(c, rstText).Height(32).Padding(0, 10).Radius(9).FontSize(12.5).Transition(colorFadeFast)
				if a.resetArmed() {
					rstBtn.TextColor(p.Danger)
				}
				if rstBtn.Clicked() {
					a.requestReset(snap)
				}
			}
		})
	})
}

func (a *App) renderTimerSetupOverlay(c *ui.Context, snap timer.Snapshot, p Palette, inStage bool) {
	t := snap.Timer
	wasAwaitPlan := t.AwaitPlan

	renderBody := func(card ui.Element) {
		card.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "选择专注计划").FontSize(15).FontWeight(700).TextColor(p.Navy)
				ui.Spacer(c)
				cls := timerBarBtn(c, IconClose, "关闭计划选择", "关闭", false, false, p).Size(26, 26)
				if cls.Clicked() {
					a.timerShowSetup = false
					a.Timer.ClearAwaitPlan()
				}
			})

			ui.Scroll(c.Key("setup-scroll")).Grow(1).MinHeight(0).Gap(10).Children(func() {
				ui.Text(c, "多轮番茄计划：专注 → 短休 → … → 长休，全程自动推进").FontSize(11.5).TextColor(p.Ink3)
				ui.Grid(c).Columns(2).Gap(8).FillWidth().Children(func() {
					for _, pl := range timer.DefaultPlans {
						active := t.Plan != nil && t.Plan.Key == pl.Key
						pc := ui.ButtonBase(c.Key("plan-"+pl.Key)).Padding(10, 11).Radius(10).
							Background(p.Surface).Border(1, p.Line).Cursor(ui.CursorPointer).
							Transition(colorFadeFast).Label(pl.Name)
						if active {
							pc.Background(p.AccentSoft).Border(1.5, p.Blue)
						} else if pc.Hovered() {
							pc.Border(1, p.Blue)
						}
						pc.Children(func() {
							ui.Column(c).Gap(2).Children(func() {
								ui.Text(c, pl.Name).FontSize(13).FontWeight(700).TextColor(p.Navy)
								ui.Textf(c, "%d 专注 + %d 短休 ×%d 轮", pl.Focus, pl.Short, pl.Rounds).FontSize(10.5).TextColor(p.Ink3)
							})
						})
						if pc.Clicked() {
							a.Timer.Command("applyPlan", timer.CmdPayload{Plan: pl.Key})
							if wasAwaitPlan {
								a.Timer.Command("start")
							}
							a.timerShowSetup = false
						}
					}
				})

				ui.Text(c, "或选单段专注").FontSize(11.5).TextColor(p.Ink3)
				ui.Row(c).Gap(6).FillWidth().Children(func() {
					for _, mins := range []int{25, 45} {
						b := ui.Button(c.Key(fmt.Sprintf("pmin-%d", mins)), fmt.Sprintf("%d 分钟", mins)).Grow(1).Padding(7, 0).FontSize(12.5)
						if t.Phase == "focus" && t.Plan == nil && t.TotalSec == mins*60 {
							b.Background(p.AccentSoft).Border(1, p.Blue)
						}
						if b.Clicked() {
							a.Timer.Command("setDuration", timer.CmdPayload{Sec: mins * 60})
							if wasAwaitPlan {
								a.Timer.Command("start")
							}
							a.timerShowSetup = false
						}
					}
				})

				ui.Row(c).Gap(6).FillWidth().Children(func() {
					sBtn := ui.Button(c, "短休 5 分钟").Grow(1).Padding(7, 0).FontSize(12.5)
					if sBtn.Clicked() {
						a.Timer.Command("setPhase", timer.CmdPayload{Phase: "short"})
						a.timerShowSetup = false
					}
					lBtn := ui.Button(c, "长休 15 分钟").Grow(1).Padding(7, 0).FontSize(12.5)
					if lBtn.Clicked() {
						a.Timer.Command("setPhase", timer.CmdPayload{Phase: "long"})
						a.timerShowSetup = false
					}
				})

				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "自定义").FontSize(12).TextColor(p.Ink2)
					in := ui.TextInput(c.Key("custom-min-in"), &a.timerCustomMin).Font(FontMono).Width(74).Placeholder("1-180").Label("自定义分钟")
					appB := ui.PrimaryButton(c, "应用").Padding(5, 12).FontSize(12)
					if in.Submitted() || appB.Clicked() {
						m, err := strconv.Atoi(a.timerCustomMin)
						if err != nil || m < 1 || m > 180 {
							c.Toast("请输入 1–180 的整数分钟")
						} else {
							a.Timer.Command("setDuration", timer.CmdPayload{Sec: m * 60})
							if wasAwaitPlan {
								a.Timer.Command("start")
							}
							a.timerShowSetup = false
						}
					}
				})
			})
		})
	}

	if inStage {
		ui.Box(c.Key("stage-setup-backdrop")).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(10).
			Background(p.ModalBackdrop).Center().Padding(24).Children(func() {
			card := ui.Column(c.Key("timer-setup-overlay")).Width(420).MaxWidthPercent(94).MaxHeight(460).
				Padding(18).Radius(16).Background(p.Paper).Border(1, p.Line).
				Shadow(0, 16, 40, 0, p.ElevatedShadow).Transition(timerOverlayMotion).Gap(10)
			if card.PressedOutside() {
				a.timerShowSetup = false
				a.Timer.ClearAwaitPlan()
			}
			renderBody(card)
		})
		return
	}

	card := ui.Column(c.Key("timer-setup-overlay")).Absolute().Top(46).Right(10).Bottom(56).Left(10).ZIndex(10).
		Padding(14).Radius(12).Background(p.Paper).Border(1, p.Line).
		Shadow(0, 8, 24, 0, p.ElevatedShadow).Transition(timerOverlayMotion).Gap(8)
	renderBody(card)
}

func (a *App) renderTimerSettingsOverlay(c *ui.Context, snap timer.Snapshot, p Palette, inStage bool) {
	renderBody := func(card ui.Element) {
		card.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "专注钟设置").FontSize(15).FontWeight(700).TextColor(p.Navy)
				ui.Spacer(c)
				cls := timerBarBtn(c, IconClose, "关闭专注钟设置", "关闭", false, false, p).Size(26, 26)
				if cls.Clicked() {
					a.timerShowSet = false
				}
			})

			ui.Scroll(c.Key("tset-scroll")).Grow(1).MinHeight(0).Gap(10).Children(func() {
				a.renderTimerPrefRows(c, snap, p, 12.5, true)
			})
		})
	}

	if inStage {
		ui.Box(c.Key("stage-set-backdrop")).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(10).
			Background(p.ModalBackdrop).Center().Padding(24).Children(func() {
			card := ui.Column(c.Key("timer-settings-overlay")).Width(420).MaxWidthPercent(94).MaxHeight(480).
				Padding(18).Radius(16).Background(p.Paper).Border(1, p.Line).
				Shadow(0, 16, 40, 0, p.ElevatedShadow).Transition(timerOverlayMotion).Gap(10)
			if card.PressedOutside() {
				a.timerShowSet = false
			}
			renderBody(card)
		})
		return
	}

	card := ui.Column(c.Key("timer-settings-overlay")).Absolute().Top(46).Right(10).Bottom(56).Left(10).ZIndex(10).
		Padding(14).Radius(12).Background(p.Paper).Border(1, p.Line).
		Shadow(0, 14, 34, 0, p.ElevatedShadow).Transition(timerOverlayMotion).Gap(8)
	renderBody(card)
}

// renderTimerPrefRows renders the focus-clock preference rows. The quick overlay in the Focus Clock
// and the Settings page share it, so every preference is reachable from the one Settings page.
func (a *App) renderTimerPrefRows(c *ui.Context, snap timer.Snapshot, p Palette, fs float32, withTheme bool) {
	prefs := snap.Prefs
	boolRow := func(label string, val bool, apply func(bool)) {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(fs).TextColor(p.Ink2).Grow(1)
			v := val
			if ui.Switch(c.Key("tsw-"+label), &v).Label(label).Changed() {
				apply(v)
			}
		})
	}
	boolRow("提示音", prefs.Sound, func(v bool) { a.Timer.SetPrefs(timer.PrefsPatch{Sound: &v}) })
	boolRow("系统通知", prefs.Notify, func(v bool) { a.Timer.SetPrefs(timer.PrefsPatch{Notify: &v}) })

	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
		ui.Text(c, "动画").FontSize(fs).TextColor(p.Ink2).Grow(1)
		mIdx := 0
		if prefs.Motion == "calm" {
			mIdx = 1
		} else if prefs.Motion == "off" {
			mIdx = 2
		}
		if ui.Segmented(c, &mIdx, "流畅", "舒缓", "关闭").Label("动画档位").Changed() {
			m := []string{"full", "calm", "off"}[mIdx]
			a.Timer.SetPrefs(timer.PrefsPatch{Motion: &m})
		}
	})

	if withTheme {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "外观").FontSize(fs).TextColor(p.Ink2).Grow(1)
			tIdx := 0
			if prefs.Theme == "dark" {
				tIdx = 1
			} else if prefs.Theme == "system" {
				tIdx = 2
			}
			if ui.Segmented(c, &tIdx, "浅色", "深色", "系统").Label("小窗外观").Changed() {
				m := []string{"light", "dark", "system"}[tIdx]
				a.Timer.SetPrefs(timer.PrefsPatch{Theme: &m})
			}
		})
	}

	boolRow("高对比度", prefs.HighContrast, func(v bool) { a.Timer.SetPrefs(timer.PrefsPatch{HighContrast: &v}) })
	boolRow("显示秒", prefs.ShowSeconds, func(v bool) { a.Timer.SetPrefs(timer.PrefsPatch{ShowSeconds: &v}) })
	boolRow("托盘常驻", prefs.Tray, func(v bool) {
		a.Timer.SetPrefs(timer.PrefsPatch{Tray: &v})
		if a.Tray != nil {
			if v {
				a.Tray.Create()
			} else {
				a.Tray.Destroy()
			}
		}
	})
	boolRow("关闭主窗时留在托盘", prefs.CloseToTray, func(v bool) { a.Timer.SetPrefs(timer.PrefsPatch{CloseToTray: &v}) })
}

func (a *App) renderTimerDoneOverlay(c *ui.Context, p Palette) {
	backdrop := ui.Column(c.Key("timer-done-backdrop")).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(10).
		Background(p.TimerDoneBg).Center().Padding(16)

	// Same-palette GPU confetti celebration burst for 1.3s upon completion (sampled live per paint frame!)
	if !a.confettiAt.IsZero() && !c.Preferences().ReduceMotion {
		startAt := a.confettiAt
		if time.Since(startAt) < 1300*time.Millisecond {
			confettiCols := []ui.Color{p.Blue, p.Blue2, p.BlueSoft, p.Run, p.BlueDeep}
			backdrop.DrawOver(func(pt *ui.Painter, r ui.Rect) {
				elapsed := time.Since(startAt)
				if elapsed < 0 || elapsed >= 1300*time.Millisecond {
					return
				}
				t := float64(elapsed.Seconds())
				alpha := float32(math.Max(0, 1.0-t/1.3))
				ox := float64(r.X + r.W/2)
				oy := float64(r.Y + r.H*0.36)
				for i := 0; i < 28; i++ {
					ang := (float64(i)/28.0)*math.Pi*2 + float64(i%3)*0.17
					spd := 70.0 + float64((i*37)%95)
					px := ox + math.Cos(ang)*spd*t
					py := oy + math.Sin(ang)*spd*t + 160.0*t*t
					sz := float32(4.5 + float64(i%3)*1.6)
					col := confettiCols[i%len(confettiCols)].Alpha(alpha)
					rot := float32(t * float64(180+(i%5)*70))
					pt.Transform(ui.Rotation(rot), float32(px), float32(py), func() {
						pt.Fill(ui.Rect{X: float32(px) - sz/2, Y: float32(py) - sz/2, W: sz, H: sz * 0.65}, col, 1.5)
					})
				}
				pt.AnimationFrameRate(60)
			})
		}
	}

	backdrop.Children(func() {
		ui.Column(c.Key("timer-done-card")).FillWidth().MaxWidth(290).Padding(20, 18).Radius(16).Background(p.Paper).
			Border(1, p.Line).Shadow(0, 12, 28, 0, p.ElevatedShadow).
			Transition(timerDoneCardMotion).Center().Gap(10).Children(func() {
			a.renderTimerDoneBody(c, p, "返回专注钟")
		})
	})
}

// renderTimerDoneBody renders the round-finished summary and next-step actions. It is shared by the
// Focus Clock / PiP overlay and by the main-window modal shown on every other page.
func (a *App) renderTimerDoneBody(c *ui.Context, p Palette, closeLabel string) {
	ev := a.timerDoneEv
	if ev == nil {
		return
	}
	emoji := "🎉"
	title := "专注完成！"
	sub := fmt.Sprintf("本轮 %d 分钟 · 今日第 %d 轮", ev.PlannedMinutes, ev.Round)
	if ev.Phase != "focus" {
		emoji = "☕"
		title = "休息结束"
		sub = fmt.Sprintf("今日已专注 %d 分钟，准备好再来一轮了吗", ev.MinutesToday)
	}
	ui.Text(c, emoji).FontSize(34)
	ui.Text(c, title).FontSize(19).FontWeight(800).TextColor(p.Navy)
	ui.Text(c, sub).FontSize(12.5).TextColor(p.Ink3)

	// Close-loop: allow checking off bound todo task directly upon finishing focus!
	if ev.Phase == "focus" && ev.TaskID != "" {
		taskDay := ev.Day
		if taskDay == "" {
			taskDay = a.Todo.TodayKey()
		}
		var boundPending bool
		for _, tk := range a.Todo.TasksForDay(taskDay) {
			if tk.ID == ev.TaskID && !tk.Done {
				boundPending = true
				break
			}
		}
		if boundPending {
			doneTaskBtn := ui.ButtonBase(c).FillWidth().Height(34).Radius(999).Center().
				Background(p.SuccessSoft).Border(1, p.SuccessLine).Transition(colorFadeFast).
				Cursor(ui.CursorPointer).Label("完成绑定任务")
			doneTaskBtn.Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconCheck).Size(14, 14).TextColor(p.Success)
					lbl := "完成此任务并打勾"
					if ev.TaskTitle != "" {
						lbl = "勾选完成：" + ev.TaskTitle
					}
					ui.Text(c, lbl).FontSize(12).FontWeight(700).TextColor(p.Success).MaxWidth(210).SingleLine()
				})
			})
			if doneTaskBtn.Clicked() {
				a.Todo.ToggleTaskInDay(taskDay, ev.TaskID)
				a.markStatsDirty()
				a.InvalidateAll()
				c.Toast("已勾选完成任务")
			}
		}
	}

	detailBtn := ui.Button(c, "查看本轮记录").Padding(4, 10).FontSize(11.5)
	if detailBtn.Clicked() {
		a.timerDoneDetail = !a.timerDoneDetail
	}
	if a.timerDoneDetail {
		pauseDesc := "中途没有暂停"
		if ev.PauseCount > 0 {
			pauseDesc = fmt.Sprintf("暂停 %d 次 · 共 %d 分钟", ev.PauseCount, (ev.PausedMs+30000)/60000)
		}
		ui.Column(c).FillWidth().Padding(8, 10).Radius(8).Background(p.TimerDoneDetail).Border(1, p.BorderSubtle).Gap(2).Children(func() {
			ui.Textf(c, "实际用时 %d 分钟 · %s", ev.ActualMinutes, pauseDesc).FontSize(12).TextColor(p.Ink3)
		})
	}

	ui.Column(c).FillWidth().Gap(6).Children(func() {
		again := ui.PrimaryButton(c, "再来一轮").FillWidth().Height(34).Radius(999)
		if again.Clicked() {
			a.timerDoneVisible = false
			a.Timer.Command("setPhase", timer.CmdPayload{Phase: "focus"})
			a.Timer.Command("start")
		}
		if ev.Phase == "focus" {
			brk := ui.Button(c, "休息 5 分钟").FillWidth().Height(34).Radius(999)
			if brk.Clicked() {
				a.timerDoneVisible = false
				a.Timer.Command("setPhase", timer.CmdPayload{Phase: "short"})
				a.Timer.Command("start")
			}
		}
		ret := ui.Button(c, closeLabel).FillWidth().Height(34).Radius(999)
		if ret.Clicked() {
			a.timerDoneVisible = false
		}
	})
}
