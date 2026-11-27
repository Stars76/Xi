package uiapp

import (
	"fmt"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/store"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo/ui"
)

// renderFocusStage renders the unified in-window immersive Focus Stage:
// full character ocean + floating rolling countdown + bound task with live subtask checkboxes + 1-click PiP pop-out.
func (a *App) renderFocusStage(c *ui.Context, snap timer.Snapshot, p Palette) {
	t := snap.Timer
	prefs := snap.Prefs

	if t.AwaitPlan && !a.timerShowSetup {
		a.timerShowSetup = true
	}

	remainRatio := 1.0
	if t.TotalSec > 0 {
		remainRatio = float64(t.LeftMs) / float64(int64(t.TotalSec)*1000)
	}

	// Resolve bound task live from its exact day bucket (works even if Todo is viewing a historical day!)
	var boundTask *store.Task
	var boundTaskDay string
	if t.Task != nil {
		boundTaskDay = t.Task.Day
		if boundTaskDay == "" {
			boundTaskDay = a.Todo.TodayKey()
		}
		for _, tk := range a.Todo.TasksForDay(boundTaskDay) {
			if tk.ID == t.Task.ID {
				cp := tk
				boundTask = &cp
				break
			}
		}
	}

	// Layer 0: Full-bleed interactive character ocean (with synchronized weather, lighting & encounters)
	sc := a.seaSceneSnapshot()
	effWeather := a.effectiveWeather()

	root := ui.Box(c).Fill().LinearGradient(ui.LinearGradient{
		From:  p.Paper,
		To:    p.Surface,
		Angle: 180,
		Oklab: true,
	}).Clip()
	px, py, pIn := root.PointerPosition()
	if root.Clicked() && !a.timerShowSetup && !a.timerShowSet && !a.sceneModalOpen && !a.timerDoneVisible {
		a.handleOceanClick(px, py)
	}
	root.Children(func() {
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
			PointerIn:    pIn && !a.timerShowSetup && !a.timerShowSet && !a.timerDoneVisible && !a.sceneModalOpen,
			Weather:      effWeather,
			TimeLighting: sc.TimeLighting,
			EncountersOn: sc.EncountersOn,
			TodayMinutes: snap.Stats.Minutes,
			TodayRounds:  snap.Stats.Rounds,
			Now:          a.liveNow(),
			OnEncounter:  a.recordNaturalEncounter,
		})

		// Layer 1: Soft top & bottom translucent veils (strictly behind foreground controls)
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).HeightPercent(18).ZIndex(1).PassThrough().
			LinearGradient(ui.LinearGradient{From: p.TimerVeilTop, To: p.TimerVeilZero, Angle: 180})
		ui.Box(c).Absolute().Bottom(0).Left(0).Right(0).HeightPercent(30).ZIndex(1).PassThrough().
			LinearGradient(ui.LinearGradient{From: p.TimerVeilZero, To: p.TimerVeilBot, Angle: 180})

		// Layer 2: Foreground immersive stage
		ui.Column(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(2).Padding(20, 32, 22, 32).Children(func() {
			// Focus Clock top header
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.RichText(c,
					ui.Span{Text: "专注", Weight: 800, Size: 24, Color: p.Ink},
					ui.Span{Text: "钟", Weight: 800, Size: 24, Color: p.Blue},
				).LetterSpacing(-0.4)

				// Status pill
				dotCol := p.IdleDot
				stText := "待机中"
				if t.Phase != "focus" {
					dotCol = p.Run
					stText = t.PhaseLabel
				} else if t.Running {
					dotCol = p.Run
					stText = "专注中"
				} else if t.Paused {
					dotCol = p.Warn
					stText = "已暂停"
				}
				ui.Row(c).Height(26).Padding(0, 10).Radius(999).Background(p.TimerChipBg).Border(1, p.BorderSubtle).
					Gap(6).AlignItems(ui.Center).Children(func() {
					dot := ui.Box(c).Size(6, 6).Radius(999).Background(dotCol)
					if t.Running {
						pulse := dot.Loop("stage-dot-pulse", 1600*time.Millisecond, ui.EaseInOut)
						dot.Opacity(0.35 + 0.65*pulse)
					}
					ui.Text(c, stText).FontSize(11.5).FontWeight(700).TextColor(p.Ink2).SingleLine()
				})

				ui.Spacer(c)

				// Ocean Scene & Encounter Atlas trigger
				sceneBadge := WeatherDisplayBadge(sc.WeatherMode, sc.LiveWeather, sc.LiveTempC)
				sceneBtn := ui.ButtonBase(c).Height(32).Padding(0, 11).Radius(9).
					Background(p.TimerChipBg).Border(1, p.Line).Transition(colorFadeFast).
					Cursor(ui.CursorPointer).Label("海域场景").Tooltip("海域场景")
				if a.sceneModalOpen {
					sceneBtn.Background(p.AccentSoft).Border(1, p.Blue)
				} else if sceneBtn.Hovered() {
					sceneBtn.Border(1, p.Blue)
				}
				sceneBtn.Children(func() {
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconScene).Size(14, 14).TextColor(p.BlueDeep)
						ui.Text(c, sceneBadge).FontSize(12.5).FontWeight(600).TextColor(p.Navy).SingleLine()
					})
				})
				if sceneBtn.Clicked() {
					a.sceneModalOpen = !a.sceneModalOpen
					a.timerShowSetup = false
					a.timerShowSet = false
				}

				// 1-click PiP pop-out buttons
				pipBtn := ui.ButtonBase(c).Height(32).Padding(0, 12).Radius(9).
					Background(p.TimerChipBg).Border(1, p.Line).Transition(colorFadeFast).
					Cursor(ui.CursorPointer).Label("弹出画中画").Tooltip("剥离为置顶字符海小窗")
				if pipBtn.Hovered() {
					pipBtn.Border(1, p.Blue)
				}
				pipBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconPiP).Size(14, 14).TextColor(p.BlueDeep)
						ui.Text(c, "画中画").FontSize(12.5).FontWeight(600).TextColor(p.Navy).SingleLine()
					})
				})
				if pipBtn.Clicked() {
					a.SetTimerMiniPill(false)
					a.OpenTimerWindow()
				}

				pillBtn := ui.ButtonBase(c).Height(32).Padding(0, 11).Radius(9).
					Background(p.TimerChipBg).Border(1, p.Line).Transition(colorFadeFast).
					Cursor(ui.CursorPointer).Label("弹出灵动条").Tooltip("剥离为 264×50 悬浮胶囊")
				if pillBtn.Hovered() {
					pillBtn.Border(1, p.Blue)
				}
				pillBtn.Children(func() {
					ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconCompact).Size(13.5, 13.5).TextColor(p.Ink2)
						ui.Text(c, "灵动条").FontSize(12.5).FontWeight(600).TextColor(p.Ink2).SingleLine()
					})
				})
				if pillBtn.Clicked() {
					a.OpenTimerWindow()
					a.SetTimerMiniPill(true)
				}

				setBtn := timerBarBtn(c, IconSettings, "专注钟设置", "专注钟设置", a.timerShowSet, false, p)
				if setBtn.Clicked() {
					a.timerShowSet = !a.timerShowSet
					a.timerShowSetup = false
				}
			})

			// Center floating numeral + primary controls
			ui.Column(c).Grow(1).FillWidth().Center().Gap(16).Children(func() {
				timeStr := dateutil.FmtDuration(t.LeftSec, prefs.ShowSeconds)
				numWrap := ui.Column(c).Center().Gap(6).Padding(14, 30)
				if prefs.HighContrast {
					numWrap.Radius(20).Background(p.TimerHCBack).Border(1, p.Line)
				} else {
					numWrap.Draw(func(pt *ui.Painter, r ui.Rect) {
						pt.FillRadialGradient(r, ui.RadialGradient{
							Stops: []ui.GradientStop{
								{Color: p.Paper.Alpha(0.78), At: 0.0},
								{Color: p.Paper.Alpha(0.45), At: 0.56},
								{Color: p.Paper.Alpha(0.0), At: 1.0},
							},
						}, 0)
					})
				}
				numWrap.Children(func() {
					weight := 600
					if prefs.HighContrast {
						weight = 800
					}
					ui.Text(c, timeStr).Font(FontNum).FontSize(92).FontWeight(weight).
						FontFeatures("tnum").TextColor(p.Navy).
						TextShadow(0, 0, 22, p.TimerNumGlow).
						ContentTransition(countdownRoll).SingleLine()
					if t.Plan != nil {
						ui.Textf(c, "%s · 第 %d / %d 段", t.Plan.Name, t.Plan.Index+1, t.Plan.Total).
							FontSize(13).FontWeight(700).TextColor(p.BlueDeep).
							TextShadow(0, 0, 12, p.TimerNumGlow)
					}
				})

				// Clean centered control pill
				setupText := fmt.Sprintf("%d 分钟", (t.TotalSec+30)/60)
				if t.Plan != nil {
					setupText = t.Plan.Name
				}
				primaryLabel := "开始专注"
				if t.Phase != "focus" {
					primaryLabel = "开始休息"
				}
				if t.Running {
					primaryLabel = "暂停"
				} else if t.Paused {
					primaryLabel = "继续"
				}

				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					planBtn := ui.ButtonBase(c).Height(40).Padding(0, 16).Radius(11).
						Background(p.TimerChipBg).Border(1, p.Line).Shadow(0, 2, 8, 0, p.CardShadow).
						Transition(colorFadeFast).Cursor(ui.CursorPointer).Label("选择计划 " + setupText)
					if planBtn.Hovered() {
						planBtn.Border(1, p.Blue)
					}
					planBtn.Children(func() {
						ui.Text(c, setupText).FontSize(13.5).FontWeight(600).TextColor(p.Ink).SingleLine()
					})
					if planBtn.Clicked() {
						a.timerShowSetup = !a.timerShowSetup
						a.timerShowSet = false
					}

					mainBtn := sheenPrimaryBtn(c, primaryLabel, 40, p).Padding(0, 24).Radius(11).Tooltip("空格")
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
						rstBtn := ui.ButtonBase(c).Height(40).Padding(0, 15).Radius(11).
							Background(p.TimerChipBg).Border(1, p.Line).Transition(colorFadeFast).
							Cursor(ui.CursorPointer).Label("重置计时")
						rstText, rstCol := "重置", p.Ink2
						if a.resetArmed() {
							rstText, rstCol = "确认放弃本轮", p.Danger
							rstBtn.Border(1, p.Danger).Tooltip("本轮会记为中断，再点一次确认")
						} else if rstBtn.Hovered() {
							rstBtn.Border(1, p.Blue)
						}
						rstBtn.Children(func() {
							ui.Text(c, rstText).FontSize(13.5).FontWeight(600).TextColor(rstCol).SingleLine()
						})
						if rstBtn.Clicked() {
							a.requestReset(snap)
						}
					}
				})
			})

			// Bottom unified Task Context Card (live subtasks checklist OR 1-click task binder!)
			ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
				if boundTask != nil {
					a.renderStageBoundTaskCard(c, boundTaskDay, *boundTask, p)
				} else {
					a.renderStageQuickBindStrip(c, p)
				}
			})
		})

		// Layer 10: Shared overlays inside Focus Clock (centered modal cards with frosted backdrop)
		if a.timerShowSetup {
			a.renderTimerSetupOverlay(c, snap, p, true)
		}
		if a.timerShowSet {
			a.renderTimerSettingsOverlay(c, snap, p, true)
		}
		if a.sceneModalOpen {
			a.renderSeaSceneModal(c, p)
		}
		if a.timerDoneVisible && a.timerDoneEv != nil {
			a.renderTimerDoneOverlay(c, p)
		}
	})
}

func (a *App) renderStageBoundTaskCard(c *ui.Context, taskDay string, task store.Task, p Palette) {
	ui.Column(c.Key("stage-bound-card")).FillWidth().MaxWidth(540).Padding(13, 16).Radius(14).
		Background(p.Paper.Alpha(0.94)).Border(1, p.Line).Shadow(0, 8, 24, 0, p.CardShadow).Gap(8).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			cb := a.circleCheck(c.Key("stg-cb-"+task.ID), task.ID, task.Done, 22, "舞台完成 "+task.Title, p)
			if cb.Clicked() {
				willDone := !task.Done
				a.Todo.ToggleTaskInDay(taskDay, task.ID)
				if willDone {
					a.checkRippleID = task.ID
					a.checkRippleAt = time.Now()
					a.stageSea.PulseTaskWave(a.now())
				}
				a.markStatsDirty()
			}
			ui.Column(c).Grow(1).Gap(1).Children(func() {
				ui.Text(c, "当前任务").FontSize(10.5).FontWeight(700).TextColor(p.BlueDeep)
				ttl := ui.Text(c, task.Title).FontSize(14.5).FontWeight(700).SingleLine()
				if task.Done {
					ttl.TextColor(p.Ink3).Strikethrough()
				} else {
					ttl.TextColor(p.Ink)
				}
			})
			if len(task.Subtasks) > 0 {
				subDone := 0
				for _, s := range task.Subtasks {
					if s.Done {
						subDone++
					}
				}
				badgeChip(c, fmt.Sprintf("%d/%d 步骤", subDone, len(task.Subtasks)), p.AccentSoft, p.BlueDeep, ui.Transparent)
			}
			unbind := ui.ButtonBase(c).Padding(4, 9).Radius(7).Background(p.Surface).Border(1, p.LineSoft).
				Transition(colorFadeFast).Cursor(ui.CursorPointer).Label("换一个任务")
			if unbind.Hovered() {
				unbind.Border(1, p.Blue)
			}
			unbind.Children(func() {
				ui.Text(c, "解绑").FontSize(11.5).FontWeight(600).TextColor(p.Ink3).SingleLine()
			})
			if unbind.Clicked() {
				a.Timer.Command("unbindTask")
			}
		})

		if len(task.Subtasks) > 0 {
			ui.Scroll(c.Key("stage-sub-scroll")).MaxHeight(108).Gap(5).Padding(4, 0, 0, 32).Children(func() {
				for _, sub := range task.Subtasks {
					ui.Row(c.Key("stg-sub-"+sub.ID)).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						scb := a.circleCheck(c.Key("stg-scb-"+sub.ID), "", sub.Done, 18, "舞台步骤 "+sub.Title, p)
						if scb.Clicked() {
							a.Todo.ToggleSubtaskInDay(taskDay, task.ID, sub.ID)
						}
						st := ui.Text(c, sub.Title).FontSize(13).Grow(1).SingleLine()
						if sub.Done {
							st.TextColor(p.Ink3).Strikethrough()
						} else {
							st.TextColor(p.Ink2)
						}
					})
				}
			})
		}
	})
}

func (a *App) renderStageQuickBindStrip(c *ui.Context, p Palette) {
	today := a.Todo.TodayKey()
	var openTasks []store.Task
	for _, tk := range a.Todo.TasksForDay(today) {
		if !tk.Done {
			openTasks = append(openTasks, tk)
		}
	}
	if len(openTasks) == 0 {
		return
	}
	// Three tasks keep the strip to one line; the rest are one click away instead of unreachable.
	const collapsedCount = 3
	hidden := 0
	if !a.stageBindAll && len(openTasks) > collapsedCount {
		hidden = len(openTasks) - collapsedCount
		openTasks = openTasks[:collapsedCount]
	}
	ui.Row(c).FillWidth().MaxWidth(620).Padding(10, 14).Radius(13).
		Background(p.Paper.Alpha(0.90)).Border(1, p.Line).Shadow(0, 6, 18, 0, p.CardShadow).
		Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, "绑定今日任务").FontSize(12).FontWeight(700).TextColor(p.Ink3).SingleLine()
		for _, tk := range openTasks {
			tCopy := tk
			pill := ui.ButtonBase(c.Key("qbind-"+tk.ID)).Height(28).Padding(0, 11).Radius(999).
				Background(p.Surface).Border(1, p.Line).Transition(colorFadeFast).
				Cursor(ui.CursorPointer).Label("绑定 " + tk.Title)
			if pill.Hovered() {
				pill.Background(p.AccentSoft).Border(1, p.Blue)
			}
			pill.Children(func() {
				ui.Text(c, tk.Title).FontSize(12).FontWeight(600).TextColor(p.Ink).MaxWidth(150).SingleLine()
			})
			if pill.Clicked() {
				a.stageBindAll = false
				a.Timer.StartTask(tCopy.ID, a.Todo.TodayKey(), false)
			}
		}
		if hidden > 0 || a.stageBindAll {
			moreText := fmt.Sprintf("其余 %d 项", hidden)
			if a.stageBindAll {
				moreText = "收起"
			}
			more := ui.ButtonBase(c.Key("qbind-more")).Height(28).Padding(0, 11).Radius(999).
				Background(p.AccentSoft).Border(1, p.AccentVeil).Transition(colorFadeFast).
				Cursor(ui.CursorPointer).Label("展开全部今日任务")
			if more.Hovered() {
				more.Border(1, p.Blue)
			}
			more.Children(func() {
				ui.Text(c, moreText).FontSize(12).FontWeight(700).TextColor(p.BlueDeep).SingleLine()
			})
			if more.Clicked() {
				a.stageBindAll = !a.stageBindAll
			}
		}
	})
}
