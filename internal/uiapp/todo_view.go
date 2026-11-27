package uiapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/store"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo/ui"
)

var (
	taskRowMotion = ui.ElementTransition{
		Duration:     170 * time.Millisecond,
		Ease:         ui.EaseOut,
		ExitDuration: 130 * time.Millisecond,
		Colors:       true,
		Enter:        &ui.Motion{Y: 6, Opacity: 0},
		Exit:         &ui.Motion{Collapse: true, Opacity: 0},
	}
	calCardMotion = ui.ElementTransition{
		Duration: 160 * time.Millisecond,
		Ease:     ui.EaseOut,
		Enter:    &ui.Motion{Scale: 0.96, Opacity: 0},
	}
)

// parseSmartTaskInput extracts an optional leading or trailing HH:MM alarm token (e.g. "写周报 16:30").
func parseSmartTaskInput(raw string) (title string, alarm string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", ""
	}
	parts := strings.Fields(s)
	if len(parts) >= 2 {
		if norm, ok := dateutil.NormalizeAlarm(parts[len(parts)-1]); ok {
			return strings.TrimSpace(strings.Join(parts[:len(parts)-1], " ")), norm
		}
		if norm, ok := dateutil.NormalizeAlarm(parts[0]); ok {
			return strings.TrimSpace(strings.Join(parts[1:], " ")), norm
		}
	}
	return s, ""
}

func (a *App) addTaskSmart(c *ui.Context, raw string) bool {
	title, alarm := parseSmartTaskInput(raw)
	if title == "" {
		return false
	}
	t := a.Todo.AddTask(title)
	if t == nil {
		return false
	}
	if alarm != "" {
		a.Todo.SetTaskAlarm(t.ID, alarm)
		c.Toast(fmt.Sprintf("已添加任务并设定 %s 闹钟", alarm))
	}
	a.markStatsDirty()
	return true
}

// circleCheck renders the 24px (or 20px for subtasks) circular check button with optional completion ripple.
func (a *App) circleCheck(c *ui.Context, taskID string, checked bool, size float32, label string, p Palette) ui.Element {
	btn := ui.ButtonBase(c).Size(size, size).Radius(999).Center().Cursor(ui.CursorPointer).
		Transition(colorFadeFast).Label(label).Checked(checked)
	hov := btn.Hovered()
	if checked {
		btn.Background(p.BlueDeep).Border(1.5, p.BlueDeep)
	} else {
		bd := p.CheckBorder
		if hov {
			bd = p.Blue
		}
		btn.Background(p.Paper).Border(1.5, bd)
	}
	if taskID != "" && a.checkRippleID == taskID {
		rippleStart := a.checkRippleAt
		if time.Since(rippleStart) < 420*time.Millisecond {
			btn.DrawOver(func(pt *ui.Painter, r ui.Rect) {
				elapsed := time.Since(rippleStart)
				if elapsed < 0 || elapsed >= 380*time.Millisecond {
					return
				}
				t := float32(elapsed.Seconds() / 0.38)
				ease := 1 - (1-t)*(1-t)*(1-t)
				cx := r.X + r.W/2
				cy := r.Y + r.H/2
				rad := (size / 2) + ease*(size*0.58)
				alpha := (1 - ease) * 0.55
				var ring ui.Path
				ring.Circle(cx, cy, rad)
				pt.StrokePath(&ring, 2.0*(1-0.4*ease), p.Blue.Alpha(alpha))
				pt.AnimationFrameRate(60)
			})
		} else {
			a.checkRippleID = ""
		}
	}
	btn.Children(func() {
		if checked {
			icSz := size * 0.54
			ui.Icon(c, IconCheck).Size(icSz, icSz).TextColor(ui.Hex("#FFFFFF"))
		}
	})
	return btn
}

func (a *App) renderTodoWorkspace(c *ui.Context, snap timer.Snapshot, p Palette) {
	viewDay := a.Todo.ViewDay()
	today := a.Todo.TodayKey()
	isToday := viewDay == today
	isFuture := viewDay > today
	tasks := a.Todo.Tasks()
	totals := a.taskTotals()

	doneCount := 0
	for _, t := range tasks {
		if t.Done {
			doneCount++
		}
	}
	totalCount := len(tasks)

	dayFocusMin := snap.Stats.Minutes
	if !isToday {
		histBlocks, _ := a.dayTimelineCached(viewDay)
		dayFocusMin = 0
		for _, hb := range histBlocks {
			if hb.Result != "abandoned" {
				dayFocusMin += hb.Minutes
			}
		}
	}

	remainRatio := 1.0
	if snap.Timer.TotalSec > 0 {
		remainRatio = float64(snap.Timer.LeftMs) / float64(int64(snap.Timer.TotalSec)*1000)
	}

	activeTaskID, activeTaskDay := "", ""
	if snap.Timer.Task != nil {
		activeTaskID = snap.Timer.Task.ID
		activeTaskDay = snap.Timer.Task.Day
	}

	openTasks := make([]store.Task, 0, len(tasks))
	doneTasks := make([]store.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.Done {
			doneTasks = append(doneTasks, t)
		} else {
			openTasks = append(openTasks, t)
		}
	}

	ui.Column(c).Fill().AlignItems(ui.Center).Background(p.Paper).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(780).Grow(1).MinHeight(0).Padding(18, 30, 0, 30).Children(func() {
			if isToday {
				// 526Hz EDITORIAL TIDE-RIBBON CARD:
				// Upper paper deck = pure razor-sharp typography, countdown & controls (zero glyphs behind text).
				// Bottom 32px edge = the tide ribbon, a miniature of the Focus Clock sea (~ ≈ ≋).
				t := snap.Timer
				prefs := snap.Prefs
				ui.Column(c.Key("today-ocean-hero")).FillWidth().Height(116).Margin(0, 0, 14, 0).Radius(16).
					Background(p.Paper).Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow).Clip().Children(func() {
					// Top pure-paper editorial row
					ui.Row(c).FillWidth().Height(84).Padding(14, 22, 6, 22).Gap(12).AlignItems(ui.Center).Children(func() {
						// Left: title + date pill + quiet status line
						ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.RichText(c,
									ui.Span{Text: "今日", Weight: 800, Size: 26, Color: p.Navy},
									ui.Span{Text: "事", Weight: 800, Size: 26, Color: p.Blue},
								).LetterSpacing(-0.5)

								ui.Row(c).Height(24).Padding(0, 3).Radius(8).Background(p.Surface).
									Border(1, p.LineSoft).Gap(2).AlignItems(ui.Center).Children(func() {
									prev := iconBtn(c, IconLeft, "前一天", "前一天", p).Size(20, 20)
									if prev.Clicked() {
										a.Todo.ShiftView(-1)
									}
									lbl := dateutil.DayLabel(viewDay, a.now().Year()) + " · 今天"
									ui.Text(c, lbl).Font(FontMono).FontSize(10.5).FontWeight(600).TextColor(p.Ink2).Padding(0, 4).SingleLine()
									next := iconBtn(c, IconRight, "后一天", "提前安排明天", p).Size(20, 20)
									if next.Clicked() {
										a.bindPickOpen = false
										a.Todo.ShiftView(1)
									}
								})
							})

							statusLine := fmt.Sprintf("待机中 · 今日专注 %d 分钟 · %d/%d 完成", snap.Stats.Minutes, doneCount, totalCount)
							if t.Running {
								if t.Task != nil && t.Task.Title != "" {
									statusLine = fmt.Sprintf("专注中 · %s", t.Task.Title)
								} else {
									statusLine = fmt.Sprintf("%s中 · 今日专注 %d 分钟", t.PhaseLabel, snap.Stats.Minutes)
								}
							} else if t.Paused {
								statusLine = fmt.Sprintf("已暂停 · 今日专注 %d 分钟 · %d/%d 完成", snap.Stats.Minutes, doneCount, totalCount)
							}
							ui.Text(c, statusLine).FontSize(12.5).TextColor(p.Ink3).SingleLine()
						})

						// Right: pure countdown on paper + clean action cluster
						ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
							timeStr := dateutil.FmtDuration(t.LeftSec, prefs.ShowSeconds)
							ui.Text(c, timeStr).Font(FontNum).FontSize(32).FontWeight(700).FontFeatures("tnum").
								TextColor(p.Navy).ContentTransition(countdownRoll).SingleLine()

							ui.Box(c).Width(1).Height(24).Background(p.LineSoft)

							ppLbl := "开始专注"
							if t.Running {
								ppLbl = "暂停"
							} else if t.Paused {
								ppLbl = "继续专注"
							}
							ppBtn := primaryBtn(c, ppLbl, 14.5, p)
							ppBtn.Tooltip("空格")
							if ppBtn.Clicked() {
								// Asks which task the round is for when nothing is bound, so history is not filled with unbound records.
								a.primaryTimerAction(snap, true)
							}

							stageBtn := ui.ButtonBase(c).Height(32).Padding(0, 11).Radius(9).
								Background(p.Surface).Border(1, p.Line).Transition(colorFadeFast).
								Cursor(ui.CursorPointer).Label("进入专注钟")
							if stageBtn.Hovered() {
								stageBtn.Background(p.AccentSoft).Border(1, p.Blue)
							}
							stageBtn.Children(func() {
								ui.Text(c, "专注钟 →").FontSize(12.5).FontWeight(600).TextColor(p.Navy).SingleLine()
							})
							if stageBtn.Clicked() {
								a.activeOverlay = "focus"
							}
						})
					})

					// Bottom 32px tide ribbon: the same glyph sea in miniature
					ribbon := ui.Box(c.Key("today-tide-ribbon")).FillWidth().Height(32).Cursor(ui.CursorPointer)
					rpx, rpy, rpIn := ribbon.PointerPosition()
					if ribbon.Clicked() {
						a.stageSea.PulseTaskWave(a.now())
					}
					a.stageSea.DrawTideRibbon(c, ribbon, SeaView{
						RemainRatio:  remainRatio,
						RemainSec:    t.LeftSec,
						Running:      t.Running,
						Paused:       t.Paused,
						Idle:         t.Idle,
						Motion:       prefs.Motion,
						HighContrast: prefs.HighContrast,
						Dark:         p.Dark,
						ReduceMotion: c.Preferences().ReduceMotion,
						PointerX:     rpx,
						PointerY:     rpy,
						PointerIn:    rpIn,
						Now:          a.liveNow(),
					})
				})
			} else {
				// Clean historical day header (no live Focus Ocean on past dates)
				ui.Row(c).FillWidth().Padding(14, 18).Margin(0, 0, 14, 0).Radius(14).
					Background(p.Surface).Border(1, p.Line).AlignItems(ui.Center).Gap(12).Children(func() {
					dayWord := "往日"
					if viewDay == dateutil.ShiftDay(today, 1) {
						dayWord = "明日"
					} else if isFuture {
						dayWord = "来日"
					}
					ui.RichText(c,
						ui.Span{Text: dayWord, Weight: 800, Size: 26, Color: p.Navy},
						ui.Span{Text: "事", Weight: 800, Size: 26, Color: p.Blue},
					).LetterSpacing(-0.5)

					ui.Row(c).Height(28).Padding(0, 4).Radius(9).Background(p.Paper).
						Border(1, p.Line).Gap(2).AlignItems(ui.Center).Children(func() {
						prev := iconBtn(c, IconLeft, "前一天", "前一天", p).Size(24, 24)
						if prev.Clicked() {
							a.Todo.ShiftView(-1)
						}
						lbl := dateutil.DayLabel(viewDay, a.now().Year())
						ui.Text(c, lbl).Font(FontMono).FontSize(11.5).FontWeight(700).TextColor(p.Navy).Padding(0, 5).SingleLine()
						next := iconBtn(c, IconRight, "后一天", "后一天", p).Size(24, 24)
						if next.Clicked() {
							a.Todo.ShiftView(1)
						}
					})

					tb := ui.ButtonBase(c).Height(28).Padding(0, 10).Radius(8).
						Background(p.AccentSoft).Border(1, p.AccentVeil).Transition(colorFadeFast).
						Cursor(ui.CursorPointer).Label("回今天")
					tb.Children(func() {
						ui.Text(c, "回今天").FontSize(12).FontWeight(700).TextColor(p.BlueDeep).SingleLine()
					})
					if tb.Clicked() {
						a.Todo.GoToday()
					}

					ui.Spacer(c)
					dayLine := fmt.Sprintf("完成 %d/%d 项 · 专注 %d 分钟", doneCount, totalCount, dayFocusMin)
					if isFuture {
						dayLine = fmt.Sprintf("已安排 %d 项", totalCount)
					}
					ui.Text(c, dayLine).FontSize(12.5).FontWeight(600).TextColor(p.Ink3).SingleLine()
				})
			}

			// Bind-a-task picker: shown when a round is started from Today without a bound task
			if isToday && a.bindPickOpen {
				a.renderBindPicker(c, openTasks, p)
			}

			// Add task row (today and future days, supports smart trailing/leading HH:MM alarm)
			if isToday || isFuture {
				placeholder := "添加新任务（末尾加 16:30 可设闹钟）"
				if isFuture {
					placeholder = "提前安排这一天的任务（末尾加 16:30 可设闹钟）"
				}
				ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Margin(0, 0, 16, 0).Children(func() {
					in := ui.TextInput(c.Key("add-task-input"), &a.newTaskDraft).
						Height(44).FontSize(15).Padding(10, 14).Radius(11).
						Background(p.Surface2).Placeholder(placeholder).Label("新任务").Grow(1).Tooltip("Ctrl+N")
					if a.focusNewTask {
						a.focusNewTask = false
						in.Focus()
					}
					addBtn := primaryBtn(c, "添加", 14.5, p).Height(44).Padding(0, 18).Radius(10)
					if in.Submitted() || addBtn.Clicked() {
						if a.addTaskSmart(c, a.newTaskDraft) {
							a.newTaskDraft = ""
						}
					}
				})
			} else {
				ui.Row(c).FillWidth().Padding(10, 14).Margin(0, 0, 14, 0).Radius(10).Background(p.Surface).Border(1, p.LineSoft).Children(func() {
					ui.Text(c, "正在查看历史日期：支持勾选完成、修改标题与步骤；新增任务与排序请在「今天」进行。").
						FontSize(12.5).TextColor(p.Ink3)
				})
			}

			// Summary & Filter row
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Margin(0, 0, 8, 0).Children(func() {
				heading := dateutil.DayLabel(viewDay, a.now().Year()) + "的任务"
				if isToday {
					heading = "今天的任务"
				}
				ui.Text(c, heading).FontSize(13).FontWeight(700).TextColor(p.Ink2).SingleLine()
				ui.Spacer(c)
				ui.Textf(c, "%02d / %02d", doneCount, len(tasks)).Font(FontMono).FontSize(12).FontWeight(600).TextColor(p.Ink3).SingleLine()
				ratio := 0.0
				if len(tasks) > 0 {
					ratio = float64(doneCount) / float64(len(tasks))
				}
				ui.Progress(c, ratio).Width(78).Height(4)
				filterLabel := "只看未完成"
				fBtn := ui.ButtonBase(c).Padding(4, 10).Radius(7).Cursor(ui.CursorPointer).
					Background(p.Paper).Border(1, p.Line).Transition(colorFadeFast).Label(filterLabel).Checked(a.onlyOpen)
				if a.onlyOpen {
					fBtn.Background(p.AccentSoft).Border(1, p.Blue)
				} else if fBtn.Hovered() {
					fBtn.Border(1, p.Blue)
				}
				fBtn.Children(func() {
					col := p.Ink3
					if a.onlyOpen {
						col = p.BlueDeep
					}
					ui.Text(c, filterLabel).FontSize(12).FontWeight(600).TextColor(col).SingleLine()
				})
				if fBtn.Clicked() {
					a.onlyOpen = !a.onlyOpen
					a.saveTodoViewPrefs()
				}
				if !isFuture && totalCount > 0 {
					rvBtn := ui.ButtonBase(c).Padding(4, 10).Radius(7).Cursor(ui.CursorPointer).
						Background(p.Paper).Border(1, p.Line).Transition(colorFadeFast).
						Label("复制当日复盘").Tooltip("复制这一天的 Markdown 复盘到剪贴板")
					if rvBtn.Hovered() {
						rvBtn.Border(1, p.Blue)
					}
					rvBtn.Children(func() {
						ui.Text(c, "复制复盘").FontSize(12).FontWeight(600).TextColor(p.Ink3).SingleLine()
					})
					if rvBtn.Clicked() {
						a.copyDailyReview(c, viewDay)
					}
				}
			})

			// Task list (open tasks on top, completed tasks folded neatly into a collapsible bottom section)
			ui.Scroll(c.Key("todo-list-scroll")).TrackScroll(&a.todoScroll).Grow(1).MinHeight(0).Padding(0, 4, 14, 0).Children(func() {
				if len(tasks) == 0 || (a.onlyOpen && len(openTasks) == 0) {
					// Serene ASCII Calm Horizon Empty State
					ui.Column(c).FillWidth().Padding(44, 24).Margin(8, 0, 0, 0).Radius(16).
						Background(p.Surface).Border(1, p.LineSoft).Center().Gap(10).Children(func() {
						ui.Box(c).Width(280).Height(56).Draw(func(pt *ui.Painter, r ui.Rect) {
							a.miniSea.DrawCalmWave(pt, r, p.Dark)
						})
						title := "暂无任务"
						detail := "在上方输入新任务，回车添加"
						if a.onlyOpen && len(tasks) > 0 {
							title = "已完成全部任务"
							detail = "今天安排的任务已全部完成"
						} else if isFuture {
							title = "这一天还没有安排"
							detail = "在上方输入任务，提前规划"
						} else if !isToday {
							title = "当日暂无任务"
							detail = "切换日期或点击「回今天」"
						}
						ui.Text(c, title).FontSize(18).FontWeight(700).TextColor(p.Navy)
						ui.Text(c, detail).FontSize(12.5).TextColor(p.Ink3)
					})
					return
				}

				for idx, task := range openTasks {
					a.renderTaskCard(c, idx, task, isToday, viewDay, totals, activeTaskID, activeTaskDay, snap, p)
				}

				if len(openTasks) == 0 && len(doneTasks) > 0 {
					ui.Row(c).FillWidth().Padding(14, 16).Margin(2, 0, 8, 0).Radius(12).
						Background(p.Surface).Border(1, p.LineSoft).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconCheck).Size(15, 15).TextColor(p.Success)
						ui.Text(c, "已完成今日全部待办").FontSize(13.5).FontWeight(600).TextColor(p.Navy)
					})
				}

				if !a.onlyOpen && len(doneTasks) > 0 {
					toggleIcon := IconRight
					if a.doneGroupOpen {
						toggleIcon = IconDown
					}
					hdr := ui.ButtonBase(c.Key("done-group-toggle")).FillWidth().Height(34).Margin(10, 0, 4, 0).
						Padding(0, 8).Radius(8).Transition(colorFadeFast).Cursor(ui.CursorPointer).
						Label(fmt.Sprintf("已完成 %d", len(doneTasks)))
					if hdr.Hovered() {
						hdr.Background(p.Surface)
					}
					hdr.Children(func() {
						ui.Row(c).Fill().Gap(7).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, toggleIcon).Size(12, 12).TextColor(p.Ink3)
							ui.Text(c, "已完成").FontSize(12.5).FontWeight(700).TextColor(p.Ink3).SingleLine()
							badgeChip(c, fmt.Sprintf("%d", len(doneTasks)), p.Surface, p.Ink3, p.LineSoft)
							ui.Box(c).Grow(1).Height(1).Background(p.LineSoft)
						})
					})
					if hdr.Clicked() {
						a.doneGroupOpen = !a.doneGroupOpen
						a.saveTodoViewPrefs()
					}
					if a.doneGroupOpen {
						for idx, task := range doneTasks {
							a.renderTaskCard(c, idx, task, isToday, viewDay, totals, activeTaskID, activeTaskDay, snap, p)
						}
					}
				}
			})
		})
	})
}

// renderBindPicker lets the user pick which open task the next focus round belongs to (or skip binding).
func (a *App) renderBindPicker(c *ui.Context, openTasks []store.Task, p Palette) {
	ui.Column(c.Key("bind-picker")).FillWidth().Padding(12, 16).Margin(0, 0, 14, 0).Radius(13).
		Background(p.AccentSoft).Border(1, p.AccentVeil).Transition(calCardMotion).Gap(9).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "这一轮专注做哪件事？").FontSize(13.5).FontWeight(700).TextColor(p.Navy).Grow(1).SingleLine()
			skip := ui.ButtonBase(c).Height(28).Padding(0, 11).Radius(8).Background(p.Paper).Border(1, p.Line).
				Transition(colorFadeFast).Cursor(ui.CursorPointer).Label("不绑定任务直接开始")
			if skip.Hovered() {
				skip.Border(1, p.Blue)
			}
			skip.Children(func() {
				ui.Text(c, "不绑定，直接开始").FontSize(12).FontWeight(600).TextColor(p.Ink2).SingleLine()
			})
			if skip.Clicked() {
				a.bindPickOpen = false
				a.Timer.Command("start")
			}
			cls := iconBtn(c, IconClose, "取消选择任务", "取消", p).Size(26, 26)
			if cls.Clicked() {
				a.bindPickOpen = false
			}
		})
		ui.Row(c).FillWidth().Gap(7).Wrap().Children(func() {
			for _, tk := range openTasks {
				tCopy := tk
				pill := ui.ButtonBase(c.Key("bindpick-"+tk.ID)).Height(30).Padding(0, 12).Radius(999).
					Background(p.Paper).Border(1, p.Line).Transition(colorFadeFast).
					Cursor(ui.CursorPointer).Label("专注 " + tk.Title)
				if pill.Hovered() {
					pill.Border(1, p.Blue)
				}
				pill.Children(func() {
					ui.Text(c, tk.Title).FontSize(12.5).FontWeight(600).TextColor(p.Ink).MaxWidth(220).SingleLine()
				})
				if pill.Clicked() {
					a.bindPickOpen = false
					if res := a.Timer.StartTask(tCopy.ID, a.Todo.TodayKey(), false); !res.OK {
						c.Toast(res.Error)
					}
				}
			}
		})
	})
}

func (a *App) renderTaskCard(c *ui.Context, idx int, task store.Task, isToday bool, viewDay string, totals map[string]int, activeID, activeDay string, snap timer.Snapshot, p Palette) {
	isCurrentFocus := activeID == task.ID && activeDay == viewDay

	row := ui.Column(c.Key(task.ID)).FillWidth().Padding(14, 12).Radius(11).
		BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Transition(taskRowMotion).Gap(8).Clip()

	hovered := row.Hovered() || row.FocusWithin()
	if isCurrentFocus {
		row.Background(p.AccentSoft).Border(1.5, p.Blue).Shadow(0, 6, 18, 0, p.BtnGlow).
			Draw(func(pt *ui.Painter, r ui.Rect) {
				// Clean 3px ocean-blue focus rail + whisper-soft bottom glow (zero glyphs colliding with task text)
				pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: 3, H: r.H}, p.Blue, 0)
				if snap.Timer.Running {
					pt.FillGradient(ui.Rect{X: r.X + 3, Y: r.Y + r.H - 14, W: r.W - 3, H: 14}, ui.LinearGradient{
						Angle: 180,
						Stops: []ui.GradientStop{
							{Color: ui.Transparent, At: 0.0},
							{Color: p.Blue.Alpha(0.10), At: 1.0},
						},
					}, 0)
				}
			})
	} else if hovered {
		row.Background(p.AccentSoft)
	}

	canSort := viewDay >= a.Todo.TodayKey()
	if canSort {
		if _, over := ui.DragOver[string](row); over {
			row.Border(1.5, p.Blue)
		}
		if draggedID, ok := ui.Drop[string](row); ok && draggedID != task.ID {
			a.Todo.MoveTask(draggedID, task.ID, false)
		}
	}

	showActs := hovered || isCurrentFocus || a.alarmTaskID == task.ID || a.subAddTaskID == task.ID || a.editingTaskID == task.ID
	canFocus := isToday && !task.Done

	toggleSubAdd := func() {
		if a.subAddTaskID == task.ID {
			a.subAddTaskID = ""
		} else {
			a.subAddTaskID = task.ID
			a.subAddDraft = ""
			delete(a.collapsed, task.ID)
		}
	}
	toggleAlarm := func() {
		if a.alarmTaskID == task.ID {
			a.alarmTaskID = ""
		} else {
			a.alarmTaskID = task.ID
			a.alarmInputStr = task.Alarm
		}
	}
	startEdit := func() {
		a.editingTaskID = task.ID
		a.editingTaskVal = task.Title
	}
	deleteTask := func() {
		removed, rIdx, ok := a.Todo.DeleteTask(task.ID)
		if !ok {
			return
		}
		a.lastUndoTask = &removed
		a.lastUndoIndex = rIdx
		a.markStatsDirty()
		c.ToastAction("已删除任务", "撤销", func() {
			if a.lastUndoTask != nil {
				a.Todo.RestoreTask(*a.lastUndoTask, a.lastUndoIndex)
				a.lastUndoTask = nil
				a.markStatsDirty()
			}
		})
	}

	// Right-click mirrors the hover action bar so every action is discoverable without hovering.
	row.PopoverMenu(func(m *ui.Menu) {
		if canFocus && m.Item("开始专注").Chosen() {
			a.StartTaskFocus(c, task)
		}
		if m.Item("添加小步骤").Chosen() {
			toggleSubAdd()
		}
		if m.Item("设置闹钟").Chosen() {
			toggleAlarm()
		}
		if m.Item("修改标题").Chosen() {
			startEdit()
		}
		m.Separator()
		if m.Item("删除任务").Chosen() {
			deleteTask()
		}
	})

	row.Children(func() {
		// Main task head row
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			// Crisp monospace index marker tying list rhythm to the ASCII character-grid world
			idxCol := p.Ink4
			if isCurrentFocus {
				idxCol = p.BlueDeep
			}
			ui.Textf(c, "%02d", idx+1).Font(FontMono).FontSize(11).FontWeight(600).TextColor(idxCol).Width(20).SingleLine()

			// Circular 24px check button with expanding completion ripple + synced sea wave pulse!
			cb := a.circleCheck(c.Key("cb-"+task.ID), task.ID, task.Done, 24, "完成 "+task.Title, p)
			if cb.Clicked() {
				willDone := !task.Done
				a.Todo.ToggleTask(task.ID)
				if willDone {
					a.checkRippleID = task.ID
					a.checkRippleAt = time.Now()
					a.stageSea.PulseTaskWave(a.now())
				}
				a.markStatsDirty()
			}

			// Title or inline edit
			if a.editingTaskID == task.ID {
				ed := ui.TextInput(c.Key("edit-task-"+task.ID), &a.editingTaskVal).FontSize(16).AutoFocus().Grow(1).Label("修改任务标题")
				saveEd := primaryBtn(c, "保存", 12.5, p).Padding(5, 12)
				cancelEd := ui.Button(c, "取消").Padding(5, 12).FontSize(12.5)
				if ed.Submitted() || saveEd.Clicked() {
					a.Todo.SetTaskTitle(task.ID, a.editingTaskVal)
					a.editingTaskID = ""
				} else if cancelEd.Clicked() {
					a.editingTaskID = ""
				}
			} else {
				titleEl := ui.Text(c, task.Title).FontSize(16.5).FontWeight(600).Grow(1).SingleLine()
				if task.Done {
					titleEl.FontWeight(500).TextColor(p.Ink3).Strikethrough()
				} else if isCurrentFocus {
					titleEl.FontWeight(700).TextColor(p.Navy).TextShadow(0, 0, 10, p.TimerNumGlow)
				} else {
					titleEl.TextColor(p.Ink)
				}
				if titleEl.Clicked() {
					if len(task.Subtasks) > 0 {
						a.collapsed[task.ID] = !a.collapsed[task.ID]
						a.saveTodoViewPrefs()
					}
				}
				if titleEl.DoubleClicked() {
					a.editingTaskID = task.ID
					a.editingTaskVal = task.Title
				}
			}

			// Live focusing countdown badge directly on the active task ribbon!
			if isCurrentFocus && a.editingTaskID != task.ID {
				liveTag := "专注中 " + dateutil.FmtClock(snap.Timer.LeftSec)
				if snap.Timer.Paused {
					liveTag = "已暂停 " + dateutil.FmtClock(snap.Timer.LeftSec)
				} else if snap.Timer.Idle {
					liveTag = "已绑定专注"
				}
				livePill := ui.ButtonBase(c).Height(24).Padding(0, 9).Radius(999).
					Background(p.Paper.Alpha(0.92)).Border(1, p.Blue).Cursor(ui.CursorPointer).Label("回到专注钟")
				livePill.Children(func() {
					ui.Text(c, liveTag).Font(FontMono).FontSize(11).FontWeight(700).TextColor(p.BlueDeep).SingleLine()
				})
				if livePill.Clicked() {
					a.activeOverlay = "focus"
				}
			}

			// Compact inline badges
			if a.editingTaskID != task.ID {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if task.RolledFrom != "" {
						badgeChip(c, "顺延", p.RolledBg, p.RolledInk, ui.Transparent).Tooltip("顺延自 " + task.RolledFrom)
					}
					if task.DailyID != "" {
						badgeChip(c, "每日", p.AccentSoft, p.BlueDeep, ui.Transparent)
					}
					if mins := totals[task.ID]; mins > 0 {
						badgeChip(c, fmt.Sprintf("%d分", mins), p.CreditBg, p.Ink2, ui.Transparent)
					}
					if len(task.Subtasks) > 0 {
						subDone := 0
						for _, s := range task.Subtasks {
							if s.Done {
								subDone++
							}
						}
						bg, fg := p.AccentSoft, p.BlueDeep
						if subDone == len(task.Subtasks) {
							bg, fg = p.DoneBadgeBg, p.DoneBadgeInk
						}
						badgeChip(c, fmt.Sprintf("%d/%d", subDone, len(task.Subtasks)), bg, fg, ui.Transparent)
					}
					if task.Alarm != "" {
						alBadge := ui.ButtonBase(c).Height(20).Padding(0, 8).Radius(999).
							Background(p.AccentSoft).Border(1, p.AccentVeil).Transition(colorFadeFast).
							Cursor(ui.CursorPointer).Label("闹钟 " + task.Alarm)
						if alBadge.Hovered() {
							alBadge.Border(1, p.Blue)
						}
						alBadge.Children(func() {
							ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, IconAlarm).Size(11, 11).TextColor(p.BlueDeep)
								ui.Text(c, task.Alarm).Font(FontMono).FontSize(11).FontWeight(600).TextColor(p.BlueDeep).SingleLine()
							})
						})
						if alBadge.Clicked() {
							toggleAlarm()
						}
					}
				})
			}

			// Start-focus stays visible on every open task today; the rest of the bar is revealed on hover
			if (showActs || canFocus) && a.editingTaskID != task.ID {
				ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
					if showActs && canSort {
						grip := ui.Box(c.Key("drag-"+task.ID)).Size(24, 28).Radius(6).Center().
							Cursor(ui.CursorGrab).Tooltip("拖动排序").Drag(task.ID)
						grip.Children(func() {
							ui.Icon(c, IconMove).Size(14, 14).TextColor(p.Ink4)
						})
					}
					if canFocus {
						tip := "开始专注"
						if isCurrentFocus {
							tip = "回到本轮专注钟"
						}
						fBtn := iconBtn(c, IconTimer, tip+" "+task.Title, tip, p).Size(28, 28)
						if isCurrentFocus {
							fBtn.Background(p.Paper.Alpha(0.88))
						}
						if fBtn.Clicked() {
							a.StartTaskFocus(c, task)
						}
					}
					if !showActs {
						return
					}
					subBtn := iconBtn(c, IconAdd, "加步骤 "+task.Title, "添加小步骤", p).Size(28, 28)
					if subBtn.Clicked() {
						toggleSubAdd()
					}
					alBtn := iconBtn(c, IconAlarm, "闹钟 "+task.Title, "设置闹钟", p).Size(28, 28)
					if alBtn.Clicked() {
						toggleAlarm()
					}
					edBtn := iconBtn(c, IconEdit, "修改 "+task.Title, "修改标题", p).Size(28, 28)
					if edBtn.Clicked() {
						startEdit()
					}
					delBtn := iconBtn(c, IconDelete, "删除 "+task.Title, "删除任务", p).Size(28, 28)
					if delBtn.Clicked() {
						deleteTask()
					}
				})
			}
		})

		// Inline alarm editor
		if a.alarmTaskID == task.ID {
			ui.Row(c).Margin(2, 0, 0, 54).Padding(6, 10).Radius(9).Background(p.Paper).
				Border(1, p.AccentVeil).Shadow(0, 2, 8, 0, p.CardShadow).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "闹钟时刻").FontSize(12).TextColor(p.Ink2).SingleLine()
				alIn := ui.TextInput(c.Key("al-in-"+task.ID), &a.alarmInputStr).Font(FontMono).Width(92).Placeholder("09:30").AutoFocus().Label("闹钟时间")
				saveB := primaryBtn(c, "保存", 12, p).Padding(4, 10)
				if alIn.Submitted() || saveB.Clicked() {
					v := strings.TrimSpace(a.alarmInputStr)
					if v != "" && !a.Todo.SetTaskAlarm(task.ID, v) {
						c.Toast("闹钟时间格式应为 HH:MM（如 08:30）")
					} else {
						if v == "" {
							a.Todo.SetTaskAlarm(task.ID, "")
						}
						a.alarmTaskID = ""
					}
				}
				if task.Alarm != "" {
					clrB := ui.Button(c, "清除").Padding(4, 10).FontSize(12)
					if clrB.Clicked() {
						a.Todo.SetTaskAlarm(task.ID, "")
						a.alarmTaskID = ""
					}
				}
				canB := ui.Button(c, "取消").Padding(4, 10).FontSize(12)
				if canB.Clicked() {
					a.alarmTaskID = ""
				}
			})
		}

		// Subtasks list with left guide line
		if !a.collapsed[task.ID] && (len(task.Subtasks) > 0 || a.subAddTaskID == task.ID) {
			ui.Column(c).Margin(4, 0, 0, 41).Padding(2, 0, 2, 22).
				BorderWidth(0, 0, 0, 1).BorderColor(p.Line).Gap(6).Children(func() {
				for _, sub := range task.Subtasks {
					subKey := task.ID + "/" + sub.ID
					srow := ui.Row(c.Key("sub-"+sub.ID)).FillWidth().Gap(9).Padding(2, 4).Radius(6).AlignItems(ui.Center)
					shov := srow.Hovered() || srow.FocusWithin() || a.editingSubKey == subKey
					srow.Children(func() {
						scb := a.circleCheck(c.Key("scb-"+sub.ID), "", sub.Done, 20, "步骤 "+sub.Title, p)
						if scb.Clicked() {
							a.Todo.ToggleSubtask(task.ID, sub.ID)
						}
						if a.editingSubKey == subKey {
							sin := ui.TextInput(c.Key("sed-"+sub.ID), &a.editingSubVal).FontSize(14).AutoFocus().Grow(1).Label("修改步骤")
							okb := primaryBtn(c, "保存", 12, p).Padding(3, 9)
							canb := ui.Button(c, "取消").Padding(3, 9).FontSize(12)
							if sin.Submitted() || okb.Clicked() {
								a.Todo.SetSubTitle(task.ID, sub.ID, a.editingSubVal)
								a.editingSubKey = ""
							} else if canb.Clicked() {
								a.editingSubKey = ""
							}
						} else {
							st := ui.Text(c, sub.Title).FontSize(14).Grow(1).SingleLine()
							if sub.Done {
								st.TextColor(p.Ink3).Strikethrough()
							} else {
								st.TextColor(p.Ink2)
							}
							if st.DoubleClicked() {
								a.editingSubKey = subKey
								a.editingSubVal = sub.Title
							}
							if shov {
								seBtn := iconBtn(c, IconEdit, "修改步骤 "+sub.Title, "修改步骤", p).Size(24, 24)
								if seBtn.Clicked() {
									a.editingSubKey = subKey
									a.editingSubVal = sub.Title
								}
								sdBtn := iconBtn(c, IconDelete, "删除步骤 "+sub.Title, "删除步骤", p).Size(24, 24)
								if sdBtn.Clicked() {
									a.Todo.DeleteSubtask(task.ID, sub.ID)
								}
							}
						}
					})
				}

				if a.subAddTaskID == task.ID {
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						sin := ui.TextInput(c.Key("sub-add-"+task.ID), &a.subAddDraft).FontSize(13.5).Placeholder("输入小步骤，回车添加").AutoFocus().Grow(1).Label("新小步骤")
						addB := primaryBtn(c, "添加", 12, p).Padding(4, 12)
						canB := ui.Button(c, "取消").Padding(4, 12).FontSize(12)
						if sin.Submitted() || addB.Clicked() {
							if a.Todo.AddSubtask(task.ID, a.subAddDraft) != nil {
								a.subAddDraft = ""
							}
						} else if canB.Clicked() {
							a.subAddTaskID = ""
						}
					})
				}
			})
		}
	})
}

func (p Palette) StrikeOrMuted() ui.Color {
	return p.Ink3
}

func (a *App) renderCalendarOverlay(c *ui.Context, p Palette) {
	if a.calMonth.IsZero() {
		now := a.now()
		a.calMonth = time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, time.Local)
	}
	y, m, _ := a.calMonth.Date()
	firstDay := time.Date(y, m, 1, 12, 0, 0, 0, time.Local)
	startOffset := int(firstDay.Weekday())
	daysInMonth := time.Date(y, m+1, 0, 12, 0, 0, 0, time.Local).Day()
	today := a.Todo.TodayKey()
	viewDay := a.Todo.ViewDay()
	nowDate := a.now()
	isCurrentMonth := y == nowDate.Year() && m == nowDate.Month()

	// Collect per-day focus minutes in this month for rich calendar badges
	monthPrefix := fmt.Sprintf("%04d-%02d-", y, int(m))
	dayFocusMin := map[string]int{}
	for _, h := range a.historyCached() {
		if h.Result == "done" && (h.Phase == "" || h.Phase == "focus") && strings.HasPrefix(h.Day, monthPrefix) {
			mins := h.ActualMinutes
			if mins <= 0 {
				mins = h.PlannedMinutes
			}
			dayFocusMin[h.Day] += mins
		}
	}

	ui.Column(c).Fill().AlignItems(ui.Center).Background(p.Paper).Children(func() {
	ui.Column(c).FillWidth().MaxWidth(780).Grow(1).MinHeight(0).Padding(PagePadTop, PagePadX, 18, PagePadX).Gap(14).Children(func() {
		ui.Row(c).FillWidth().Height(40).AlignItems(ui.Center).Gap(10).Children(func() {
			plainTitle(c, "日历", p)
			ui.Text(c, "点选某一天，查看或安排那天的事").FontSize(12.5).TextColor(p.Ink4).Padding(6, 0, 0, 2)
		})
		cardLook(ui.Column(c.Key("cal-card")).FillWidth(), p).
			Transition(calCardMotion).Clip().Children(func() {
			// Clean Paper & Ocean-Blue Calendar Header
			ui.Row(c).FillWidth().Height(64).Padding(0, 22).AlignItems(ui.Center).
				Background(p.Surface).BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Gap(8).Children(func() {
				prevM := iconBtn(c, IconLeft, "上个月", "上个月", p)
				if prevM.Clicked() {
					a.calMonth = a.calMonth.AddDate(0, -1, 0)
				}
				ui.Spacer(c)
				ui.Textf(c, "%d年 %02d月", y, int(m)).Font(FontNum).FontSize(22).FontWeight(700).TextColor(p.Navy)
				if !isCurrentMonth {
					jumpToday := ui.ButtonBase(c).Height(24).Padding(0, 9).Radius(999).
						Background(p.AccentSoft).Border(1, p.AccentVeil).Transition(colorFadeFast).
						Cursor(ui.CursorPointer).Label("回到本月")
					if jumpToday.Hovered() {
						jumpToday.Border(1, p.Blue)
					}
					jumpToday.Children(func() {
						ui.Text(c, "本月").FontSize(11).FontWeight(700).TextColor(p.BlueDeep).SingleLine()
					})
					if jumpToday.Clicked() {
						a.calMonth = time.Date(nowDate.Year(), nowDate.Month(), 1, 12, 0, 0, 0, time.Local)
					}
				}
				ui.Spacer(c)
				nextM := iconBtn(c, IconRight, "下个月", "下个月", p)
				if nextM.Clicked() {
					a.calMonth = a.calMonth.AddDate(0, 1, 0)
				}
			})

			ui.Column(c).FillWidth().Padding(16, 22, 22, 22).Gap(10).Children(func() {
				ui.Grid(c).Columns(7).Gap(6).FillWidth().Children(func() {
					for _, wd := range []string{"日", "一", "二", "三", "四", "五", "六"} {
						ui.Box(c).Center().PaddingY(4).Children(func() {
							ui.Text(c, wd).Font(FontMono).FontSize(12).FontWeight(600).TextColor(p.Ink3)
						})
					}
				})

				ui.Grid(c).Columns(7).Gap(6).FillWidth().Children(func() {
					for i := 0; i < 42; i++ {
						dayNum := i - startOffset + 1
						if dayNum < 1 || dayNum > daysInMonth {
							ui.Box(c).Height(52)
							continue
						}
						d := time.Date(y, m, dayNum, 12, 0, 0, 0, time.Local)
						dk := d.Format("2006-01-02")
						isTodayCell := dk == today
						isSel := dk == viewDay
						dayTasks := a.Todo.TasksForDay(dk)
						hasTasks := len(dayTasks) > 0
						allDone := hasTasks
						for _, dt := range dayTasks {
							if !dt.Done {
								allDone = false
								break
							}
						}
						fMin := dayFocusMin[dk]

						cell := ui.ButtonBase(c.Key("cal-"+dk)).Height(52).Radius(12).Center().
							Transition(colorFadeFast).Cursor(ui.CursorPointer).Label(dk)
						if isTodayCell {
							cell.Background(p.BlueDeep).Shadow(0, 4, 12, 0, p.BtnGlow)
						} else if isSel {
							cell.Background(p.AccentSoft).Border(2, p.BlueDeep)
						} else if cell.Hovered() {
							cell.Background(p.AccentSoft).Border(1, p.AccentVeil)
						}
						cell.Children(func() {
							ui.Column(c).Center().Gap(2).Children(func() {
								tc := p.Ink2
								if isTodayCell {
									tc = ui.Hex("#FFFFFF")
								} else if isSel {
									tc = p.BlueDeep
								}
								ui.Textf(c, "%d", dayNum).Font(FontNum).FontSize(15.5).FontWeight(600).TextColor(tc)
								if fMin > 0 {
									mc := p.BlueDeep
									if isTodayCell {
										mc = ui.Hex("#FFFFFF").Alpha(0.90)
									}
									ui.Textf(c, "%dm", fMin).Font(FontMono).FontSize(9.5).FontWeight(600).TextColor(mc).SingleLine()
								} else if hasTasks {
									dc := p.Blue
									if allDone {
										dc = p.Success
									}
									if isTodayCell {
										dc = ui.Hex("#FFFFFF")
									}
									ui.Box(c).Size(5, 5).Radius(999).Background(dc)
								} else {
									ui.Box(c).Size(5, 11)
								}
							})
						})
						if cell.Clicked() {
							a.Todo.SetViewDay(dk)
							a.activeOverlay = ""
						}
					}
				})
			})
		})

	})
	})
}
