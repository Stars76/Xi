package uiapp

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	"github.com/Stars76/526Hz/internal/assets"
	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/stats"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo/ui"
)

var (
	colorFadeFast = ui.ElementTransition{
		Animation: &ui.Animation{Duration: 150 * time.Millisecond, Ease: ui.EaseOut, Oklab: true, NonMotion: true},
		Colors:    true,
	}
	cardFadeFast = ui.ElementTransition{
		Animation: &ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut, Oklab: true, NonMotion: true},
		Colors:    true,
		Shadows:   true,
	}
	drawerTransition = ui.ElementTransition{
		Duration:     190 * time.Millisecond,
		Ease:         ui.EaseOut,
		ExitDuration: 140 * time.Millisecond,
		Enter:        &ui.Motion{X: 28, Opacity: 0},
		Exit:         &ui.Motion{X: 24, Opacity: 0},
	}
)

var cachedLogo ui.ImageSource

func logoSource() ui.ImageSource {
	if cachedLogo != nil {
		return cachedLogo
	}
	if src, derr := ui.DecodeImage(assets.AppIconPNG); derr == nil {
		cachedLogo = src
	}
	return cachedLogo
}

func marshalPretty(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func iconBtn(c *ui.Context, icon *ui.SVG, label, tip string, p Palette) ui.Element {
	b := ui.ButtonBase(c).Size(32, 32).Radius(8).Center().Label(label).Tooltip(tip).
		Cursor(ui.CursorPointer).Transition(colorFadeFast)
	hov := b.Hovered()
	if hov {
		b.Background(p.AccentSoft)
	}
	b.Children(func() {
		col := p.Ink3
		if hov {
			col = p.BlueDeep
		}
		ui.Icon(c, icon).Size(17, 17).TextColor(col)
	})
	return b
}

func badgeChip(c *ui.Context, text string, bg, fg, border ui.Color) ui.Element {
	el := ui.Row(c).Height(20).Padding(0, 8).Radius(999).Background(bg).AlignItems(ui.Center)
	if border != ui.Transparent {
		el.Border(1, border)
	}
	el.Children(func() {
		ui.Text(c, text).FontSize(11).FontWeight(700).FontFeatures("tnum").TextColor(fg).SingleLine()
	})
	return el
}

// MainView renders the unified 526Hz studio shell:
// Ocean-Tide Sidebar (196px, or a 56px icon rail) + Center Workspace (Today / Focus Clock / Calendar / Stats / Settings) + Optional Right Timeline Drawer.
func (a *App) MainView(c *ui.Context) {
	th, p := a.paletteOf(c)
	c.SetTheme(th)
	a.consumePendingToast(c)

	if c.Shortcut(0, ui.KeyEscape) {
		switch {
		case a.importOpen:
			a.importOpen = false
		case a.clearAskOpen:
			a.clearAskOpen = false
		case a.sceneModalOpen:
			a.sceneModalOpen = false
		case a.timerShowSet:
			a.timerShowSet = false
		case a.timerShowSetup:
			a.timerShowSetup = false
			a.Timer.ClearAwaitPlan()
		case a.timerDoneVisible:
			a.timerDoneVisible = false
		case a.bindPickOpen:
			a.bindPickOpen = false
		case a.alarmTaskID != "":
			a.alarmTaskID = ""
		case a.editingTaskID != "":
			a.editingTaskID = ""
		case a.editingSubKey != "":
			a.editingSubKey = ""
		case a.subAddTaskID != "":
			a.subAddTaskID = ""
		case a.tlFormOpen:
			a.tlFormOpen = false
		case a.tlActionKey != "":
			a.tlActionKey = ""
		case a.activeOverlay != "":
			a.activeOverlay = ""
		}
	}

	snap := a.Timer.Snapshot()
	a.handleGlobalShortcuts(c, snap)

	// Responsive shell: the sidebar folds to an icon rail (by choice, or when the window is narrow)
	// and the timeline drawer only opens when the workspace keeps a usable width beside it.
	winW, _ := c.Size()
	rail := a.sidebarCollapsed || winW < railBelowWidth
	drawerFits := winW >= drawerMinWidth
	bar := c.TitleBar()
	leftPad := float32(14)
	if runtime.GOOS == "darwin" && bar.Left > 0 {
		leftPad = bar.Left + 8
	}

	ui.Column(c).Fill().Background(p.Paper).Children(func() {
		// Sleek 48px studio titlebar
		tb := ui.Row(c).Height(48).Padding(0, 0, 0, 0).AlignItems(ui.Center).
			Background(p.Surface).BorderWidth(0, 0, 1, 0).BorderColor(p.Line).DragWindow()
		if runtime.GOOS != "darwin" && tb.WindowDoubleClicked() && a.MainWin != nil {
			a.MainWin.ToggleMaximize()
		}
		tb.Children(func() {
			// Brand zone + collapsible sidebar toggle
			brandW := float32(sidebarWidth)
			if rail {
				brandW = 160
			}
			brandRow := ui.Row(c).Width(brandW).FillHeight().Padding(0, 10, 0, leftPad).Gap(7).AlignItems(ui.Center)
			if !rail {
				brandRow.BorderWidth(0, 1, 0, 0).BorderColor(p.LineSoft)
			}
			brandRow.Children(func() {
				sbTip := "展开 / 收起导航侧栏"
				if winW < railBelowWidth {
					sbTip = "窗口较窄，侧栏已自动收为图标栏"
				}
				sbBtn := iconBtn(c, IconSidebar, "切换侧栏", sbTip, p).Size(28, 28)
				if sbBtn.Clicked() {
					a.sidebarCollapsed = !a.sidebarCollapsed
					a.saveTodoViewPrefs()
				}
				if logo := logoSource(); logo != nil {
					ui.Image(c, logo).Size(20, 20).Radius(5)
				}
				ui.RichText(c,
					ui.Span{Text: "526", Weight: 800, Size: 16, Color: p.Navy},
					ui.Span{Text: "Hz", Weight: 800, Size: 16, Color: p.Blue},
				).Font(FontNum).LetterSpacing(-0.2)
			})

			ui.Spacer(c)

			// Right timeline drawer toggle + live timer status chip (opens in-window Focus Stage or PiP!)
			ui.Row(c).Gap(8).Padding(0, 10, 0, 0).AlignItems(ui.Center).Children(func() {
				tlBtn := iconBtn(c, IconTimeline, "切换今日时间轴侧栏", "展开 / 收起今日时间轴", p)
				if a.tlDrawerOpen && drawerFits {
					tlBtn.Background(p.AccentSoft).Border(1, p.AccentVeil)
				}
				if tlBtn.Clicked() {
					if drawerFits {
						a.tlDrawerOpen = !a.tlDrawerOpen
						if a.tlDrawerOpen {
							a.markStatsDirty()
						}
						a.saveTodoViewPrefs()
					} else {
						// No room for a side drawer: open the full day timeline instead.
						a.statsTab = 0
						a.navigate("stats")
					}
				}
				a.renderShellTimerChip(c, snap, p)
			})

			// Edge-to-edge Windows caption controls
			if runtime.GOOS != "darwin" {
				ui.Row(c).FillHeight().AlignItems(ui.Center).Children(func() {
					minBtn := ui.ButtonBase(c).Width(42).FillHeight().Center().Label("最小化").Tooltip("最小化").
						Transition(colorFadeFast).Cursor(ui.CursorPointer)
					if minBtn.Hovered() {
						minBtn.Background(p.AccentSoft)
					}
					minBtn.Children(func() {
						ui.Icon(c, IconMin).Size(12, 12).TextColor(p.Ink2)
					})
					if minBtn.Clicked() && a.MainWin != nil {
						a.MainWin.Minimize()
					}

					isMax := a.MainWin != nil && a.MainWin.IsMaximized()
					maxIcon := IconMax
					if isMax {
						maxIcon = IconRestore
					}
					maxBtn := ui.ButtonBase(c).Width(42).FillHeight().Center().Label("最大化").Tooltip("最大化 / 还原").
						Transition(colorFadeFast).Cursor(ui.CursorPointer)
					if maxBtn.Hovered() {
						maxBtn.Background(p.AccentSoft)
					}
					maxBtn.Children(func() {
						ui.Icon(c, maxIcon).Size(12, 12).TextColor(p.Ink2)
					})
					if maxBtn.Clicked() && a.MainWin != nil {
						a.MainWin.ToggleMaximize()
					}

					closeTip := "关闭"
					if snap.Prefs.CloseToTray {
						closeTip = "关闭（会留在系统托盘继续计时）"
					}
					clsBtn := ui.ButtonBase(c).Width(44).FillHeight().Center().Label("关闭").Tooltip(closeTip).
						Transition(colorFadeFast).Cursor(ui.CursorPointer)
					clsHov := clsBtn.Hovered()
					if clsHov {
						clsBtn.Background(ui.Hex("#C42B35"))
					}
					clsBtn.Children(func() {
						col := p.Ink2
						if clsHov {
							col = ui.Hex("#FFFFFF")
						}
						ui.Icon(c, IconClose).Size(12, 12).TextColor(col)
					})
					if clsBtn.Clicked() && a.MainWin != nil {
						a.MainWin.Close()
					}
				})
			} else {
				ui.Box(c).Width(10)
			}
		})

		// Studio body: Collapsible 196px Ocean-Tide Sidebar + Center Workspace + Optional Right Timeline Drawer
		ui.Row(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
			a.renderSidebar(c, snap, p, rail)

			// Center unified workspace
			ui.Column(c).Grow(1).MinHeight(0).MinWidth(0).FillHeight().Background(p.Paper).Clip().Children(func() {
				switch a.activeOverlay {
				case "focus":
					a.renderFocusStage(c, snap, p)
				case "calendar":
					a.renderCalendarOverlay(c, p)
				case "settings":
					a.renderSettingsPanel(c, snap, p)
				case "stats":
					a.renderStatsPanel(c, p)
				default:
					a.renderTodoWorkspace(c, snap, p)
				}
			})

			// Optional right-hand live Today Timeline drawer
			if a.tlDrawerOpen && drawerFits && a.activeOverlay == "" {
				a.renderRightTimelineDrawer(c, p)
			}
		})
	})

	a.renderImportModal(c, p)
	a.renderClearAlert(c)
	// The Focus Clock page draws its own completion overlay; every other page gets it as a modal
	// so a finished round is never silent while the user is on Today / Stats / Settings.
	if a.timerDoneVisible && a.timerDoneEv != nil && a.activeOverlay != "focus" {
		ui.Modal(c, &a.timerDoneVisible, func() {
			ui.Column(c.Key("shell-done-card")).Width(290).Center().Gap(10).Children(func() {
				a.renderTimerDoneBody(c, p, "知道了")
			})
		})
	}
}

const (
	sidebarWidth   = 196
	railWidth      = 56
	railBelowWidth = 900  // below this window width the sidebar always shows as the icon rail
	drawerMinWidth = 1000 // the 280px timeline drawer needs at least this window width
)

// navPages lists the main-window pages in sidebar order; Ctrl+1..5 follow the same order.
var navPages = []struct {
	ID    string
	Label string
	Icon  *ui.SVG
}{
	{"", "今日事", IconList},
	{"focus", "专注钟", IconTimer},
	{"calendar", "日历", IconCalendar},
	{"stats", "统计", IconChart},
	{"settings", "设置与备份", IconSettings},
}

// handleGlobalShortcuts wires the keyboard-first paths: Ctrl+1..5 pages, Ctrl+N new task,
// Space start / pause. Text inputs and focused buttons consume their own keys first.
func (a *App) handleGlobalShortcuts(c *ui.Context, snap timer.Snapshot) {
	if a.overlayOpen() {
		return
	}
	for i, pg := range navPages {
		if c.Shortcut(ui.Ctrl, ui.Key1+ui.Key(i)) {
			a.navigate(pg.ID)
		}
	}
	if c.Shortcut(ui.Ctrl, ui.KeyN) {
		a.navigate("")
		a.Todo.GoToday()
		a.focusNewTask = true
	}
	if c.Shortcut(0, ui.KeySpace) && !c.ShortcutRepeated() {
		a.primaryTimerAction(snap, a.activeOverlay == "")
	}
}

func (a *App) renderSidebar(c *ui.Context, snap timer.Snapshot, p Palette, rail bool) {
	_, doneCount, totalCount := a.todayTideLevel(snap)
	openCount := totalCount - doneCount

	if rail {
		a.renderSidebarRail(c, snap, openCount, p)
		return
	}

	// Clean minimalist studio sidebar matching the quiet 526Hz paper-and-ocean aesthetic
	ui.Column(c).Width(sidebarWidth).FillHeight().Background(p.Surface).
		BorderWidth(0, 1, 0, 0).BorderColor(p.Line).
		Padding(14, 10, 12, 10).Gap(5).Children(func() {

			navItem := func(id, label string, icon *ui.SVG, badge string, badgeLive bool, hint string) {
				active := a.activeOverlay == id
				item := ui.ButtonBase(c.Key("nav-"+id)).FillWidth().Height(38).Radius(10).
					Padding(0, 11).Transition(cardFadeFast).Cursor(ui.CursorPointer).Label(label).Tooltip(hint)
				hov := item.Hovered()
				if active {
					item.Background(p.Paper).Border(1, p.Line).Shadow(0, 2, 8, 0, p.CardShadow)
				} else if hov {
					item.Background(p.AccentSoft)
				}
				item.Children(func() {
					ui.Row(c).Fill().Gap(9).AlignItems(ui.Center).Children(func() {
						col := p.Ink2
						weight := 600
						if active {
							col = p.BlueDeep
							weight = 700
						} else if hov {
							col = p.Navy
						}
						ui.Icon(c, icon).Size(16.5, 16.5).TextColor(col)
						ui.Text(c, label).FontSize(14).FontWeight(weight).TextColor(col).Grow(1).SingleLine()
						if badge != "" {
							bg, fg := p.CreditBg, p.Ink2
							if badgeLive {
								bg, fg = p.SuccessSoft, p.Success
							} else if active {
								bg, fg = p.AccentSoft, p.BlueDeep
							}
							badgeChip(c, badge, bg, fg, ui.Transparent)
						}
					})
				})
				if item.Clicked() {
					a.navigate(id)
				}
			}

			openBadge := ""
			if openCount > 0 {
				openBadge = fmt.Sprint(openCount)
			}
			focusBadge := ""
			if snap.Timer.Running || snap.Timer.Paused {
				focusBadge = dateutil.FmtClock(snap.Timer.LeftSec)
			}

			for i, pg := range navPages {
				badge, live := "", false
				switch pg.ID {
				case "":
					badge = openBadge
				case "focus":
					badge, live = focusBadge, snap.Timer.Running
				}
				navItem(pg.ID, pg.Label, pg.Icon, badge, live, fmt.Sprintf("Ctrl+%d", i+1))
			}

			ui.Spacer(c)

			// Crisp "今日专注" summary card anchored cleanly at the bottom of the sidebar
			ui.Column(c.Key("sidebar-summary")).FillWidth().Padding(12, 11).Radius(13).
				Background(p.Paper).Border(1, p.Line).
				Shadow(0, 4, 14, 0, p.CardShadow).Gap(8).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Text(c, "今日专注").FontSize(12).FontWeight(700).TextColor(p.Navy).Grow(1)
					ui.Textf(c, "%d/%d", doneCount, totalCount).Font(FontMono).FontSize(11).FontWeight(600).TextColor(p.Ink3).SingleLine()
				})
				ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Textf(c, "%d分 · %d轮", snap.Stats.Minutes, snap.Stats.Rounds).
						Font(FontMono).FontSize(11).TextColor(p.Ink2).Grow(1).SingleLine()
				})
				// Pure at-a-glance summary: the pop-out windows are opened from the Focus Clock toolbar.
				ratio := 0.0
				if totalCount > 0 {
					ratio = float64(doneCount) / float64(totalCount)
				}
				ui.Progress(c, ratio).FillWidth().Height(4)
			})
		})
}

// renderSidebarRail is the 56px icon-only sidebar: navigation stays one click away when the sidebar
// is collapsed or the window is too narrow for the full one.
func (a *App) renderSidebarRail(c *ui.Context, snap timer.Snapshot, openCount int, p Palette) {
	ui.Column(c).Width(railWidth).FillHeight().Background(p.Surface).
		BorderWidth(0, 1, 0, 0).BorderColor(p.Line).
		Padding(14, 0, 12, 0).Gap(5).AlignItems(ui.Center).Children(func() {
		for i, pg := range navPages {
			active := a.activeOverlay == pg.ID
			item := ui.ButtonBase(c.Key("rail-"+pg.ID)).Size(38, 38).Radius(10).Center().
				Transition(cardFadeFast).Cursor(ui.CursorPointer).Label(pg.Label).
				Tooltip(fmt.Sprintf("%s · Ctrl+%d", pg.Label, i+1))
			hov := item.Hovered()
			if active {
				item.Background(p.Paper).Border(1, p.Line).Shadow(0, 2, 8, 0, p.CardShadow)
			} else if hov {
				item.Background(p.AccentSoft)
			}
			// A small dot carries what the full sidebar shows as a badge: open tasks / a live round.
			dot := ui.Transparent
			switch {
			case pg.ID == "focus" && snap.Timer.Running:
				dot = p.Success
			case pg.ID == "focus" && snap.Timer.Paused:
				dot = p.Warn
			case pg.ID == "" && openCount > 0:
				dot = p.Blue
			}
			item.Children(func() {
				col := p.Ink2
				if active {
					col = p.BlueDeep
				} else if hov {
					col = p.Navy
				}
				ui.Icon(c, pg.Icon).Size(17, 17).TextColor(col)
				if dot != ui.Transparent {
					ui.Box(c).Absolute().Top(6).Right(6).Size(6, 6).Radius(999).Background(dot)
				}
			})
			if item.Clicked() {
				a.navigate(pg.ID)
			}
		}
	})
}

func (a *App) renderRightTimelineDrawer(c *ui.Context, p Palette) {
	today := a.Todo.TodayKey()
	blocks, _ := a.dayTimelineCached(today)
	totalM := 0
	for _, b := range blocks {
		totalM += b.Minutes
	}

	ui.Column(c.Key("right-tl-drawer")).Width(280).FillHeight().Padding(18, 16, 14, 16).
		Background(p.Surface).BorderWidth(0, 0, 0, 1).BorderColor(p.Line).
		Transition(drawerTransition).Gap(12).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "今日时间轴").FontSize(16).FontWeight(700).TextColor(p.Navy)
				ui.Textf(c, "共 %d 段 · %d 分钟", len(blocks), totalM).Font(FontMono).FontSize(12).TextColor(p.Ink3)
			})
			cls := iconBtn(c, IconClose, "收起时间轴", "收起", p).Size(28, 28)
			if cls.Clicked() {
				a.tlDrawerOpen = false
				a.saveTodoViewPrefs()
			}
		})

		ui.Scroll(c.Key("right-tl-list")).Grow(1).MinHeight(0).Gap(8).Children(func() {
			if len(blocks) == 0 {
				ui.Column(c).FillWidth().Padding(32, 12).Center().Gap(8).Children(func() {
					ui.Box(c).Size(140, 42).Draw(func(pt *ui.Painter, r ui.Rect) {
						a.miniSea.DrawCalmWave(pt, r, p.Dark)
					})
					ui.Text(c, "今天还没有专注时段").FontSize(13).FontWeight(600).TextColor(p.Ink3)
					ui.Text(c, "进入专注钟或点击下方补录").FontSize(12).TextColor(p.Ink4)
				})
				return
			}
			for idx, b := range blocks {
				bCopy := b
				accent := p.Blue
				bg := p.Paper
				bd := p.Line
				dur := fmt.Sprintf("%d分", bCopy.Minutes)
				if bCopy.Result == "abandoned" {
					accent = p.Warn
					bg = p.TlWarnBg
					bd = p.TlWarnBorder
					dur = fmt.Sprintf("中断 %d分", bCopy.Minutes)
				}
				card := ui.ButtonBase(c.Key(fmt.Sprintf("rtblk-%d-%s", idx, bCopy.Key))).FillWidth().Radius(10).
					Background(bg).Border(1, bd).Shadow(0, 2, 6, 0, p.CardShadow).Transition(cardFadeFast).
					Cursor(ui.CursorPointer).Clip().Label("查看时段 " + bCopy.Title).Tooltip("点击在完整时间轴中定位或删除")
				if card.Hovered() {
					card.Border(1, p.Blue)
				}
				card.Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Stretch).Children(func() {
						ui.Box(c).Width(3).FillHeight().Background(accent)
						ui.Column(c).Grow(1).Padding(8, 10).Gap(3).Children(func() {
							ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
								ui.Text(c, fmt.Sprintf("%s–%s", stats.ToHhmm(bCopy.StartMin), stats.ToHhmm(bCopy.EndMin))).
									Font(FontMono).FontSize(11).FontWeight(600).TextColor(p.BlueDeep).SingleLine()
								ui.Spacer(c)
								ui.Text(c, dur).Font(FontMono).FontSize(11).TextColor(p.Ink3).SingleLine()
							})
							ui.Text(c, bCopy.Title).FontSize(13).FontWeight(600).TextColor(p.Ink).SingleLine()
						})
					})
				})
				if card.Clicked() {
					a.tlDayKey = today
					a.statsTab = 0
					a.tlActionKey = bCopy.Key
					a.tlScrolledDay = ""
					a.markStatsDirty()
					a.activeOverlay = "stats"
				}
			}
		})

		fullBtn := ui.Button(c, "查看完整 24h 时间轴").FillWidth().Height(34).Radius(9).FontSize(13)
		if fullBtn.Clicked() {
			a.tlDayKey = today
			a.statsTab = 0
			a.tlScrolledDay = ""
			a.markStatsDirty()
			a.activeOverlay = "stats"
		}
	})
}

func (a *App) renderShellTimerChip(c *ui.Context, snap timer.Snapshot, p Palette) {
	t := snap.Timer
	// On Today and Focus Clock pages while idle, the page header already displays timer state — keep the titlebar uncluttered.
	if (a.activeOverlay == "" || a.activeOverlay == "focus") && t.Idle && !t.Running && !t.Paused && t.Phase == "focus" {
		return
	}
	dotCol := p.IdleDot
	chipBg := p.Paper
	chipBorder := p.Line
	textCol := p.Ink2
	statusText := "待机中"

	totalText := dateutil.FmtDuration(t.TotalSec, true)
	if t.Phase != "focus" {
		dotCol = p.Blue
		chipBg = p.AccentSoft
		chipBorder = p.AccentVeil
		textCol = p.BlueDeep
		label := "长休息"
		if t.Phase == "short" {
			label = "短休息"
		}
		if !t.Running && !t.Idle {
			statusText = "休息已暂停 · " + totalText
		} else {
			statusText = label + " · " + totalText
		}
	} else if t.Running {
		dotCol = p.Success
		chipBg = p.SuccessSoft
		chipBorder = p.SuccessLine
		textCol = p.Success
		if t.Recovered {
			statusText = "恢复中 · " + totalText
		} else {
			statusText = "专注中 · " + dateutil.FmtClock(t.LeftSec)
		}
	} else if t.Paused {
		dotCol = p.Warn
		chipBg = p.WarnBg
		chipBorder = p.WarnLine
		textCol = p.WarnInk
		statusText = "已暂停 · " + dateutil.FmtClock(t.LeftSec)
	}

	chip := ui.ButtonBase(c).Height(32).Padding(0, 12).Radius(999).Background(chipBg).Border(1, chipBorder).
		Transition(colorFadeFast).Cursor(ui.CursorPointer).Label("打开专注钟状态条").Tooltip("打开专注钟（再次点击弹出画中画）")
	if chip.Hovered() {
		chip.Border(1, p.Blue)
	}
	chip.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			dot := ui.Box(c).Size(6.5, 6.5).Radius(999).Background(dotCol)
			if t.Running {
				pulse := dot.Loop("shell-dot-pulse", 1600*time.Millisecond, ui.EaseInOut)
				dot.Opacity(0.35 + 0.65*pulse)
			}
			if t.Task != nil && t.Task.Title != "" {
				ui.Text(c, t.Task.Title).FontSize(13).FontWeight(700).TextColor(p.Navy).MaxWidth(120).SingleLine()
				ui.Text(c, "·").FontSize(13).TextColor(p.Ink4)
			}
			ui.Text(c, statusText).Font(FontMono).FontSize(12).FontWeight(700).TextColor(textCol).SingleLine()
		})
	})
	if chip.Clicked() {
		if a.activeOverlay == "focus" {
			a.SetTimerMiniPill(false)
			a.OpenTimerWindow()
		} else {
			a.activeOverlay = "focus"
		}
	}
}
