package uiapp

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/notes"
	"github.com/Stars76/526Hz/internal/stats"
	"github.com/Stars76/526Hz/internal/store"
	"github.com/egoist/mygo/ui"
)

const (
	tlHourHeight   = float32(60)
	tlMinBlockH    = float32(26)
	tlVertGap      = float32(3)
	tlMinVisualMin = 29
)

func (a *App) loadOverrides() store.TimelineOverrides {
	a.mu.Lock()
	if !a.overridesDirty {
		ov := a.cachedOverrides
		a.mu.Unlock()
		return ov
	}
	a.mu.Unlock()

	var ov store.TimelineOverrides
	if a.Store.GetJSON(store.KeyTimelineOverrides, &ov) {
		if ov.Edits == nil {
			ov.Edits = []store.TimelineEdit{}
		}
		if ov.Manual == nil {
			ov.Manual = []store.ManualBlock{}
		}
	} else {
		ov = store.TimelineOverrides{Edits: []store.TimelineEdit{}, Manual: []store.ManualBlock{}}
	}

	a.mu.Lock()
	a.cachedOverrides = ov
	a.overridesDirty = false
	a.mu.Unlock()
	return ov
}

func (a *App) saveOverrides(ov store.TimelineOverrides) {
	a.Store.SetJSON(store.KeyTimelineOverrides, ov)
	a.mu.Lock()
	a.cachedOverrides = ov
	a.overridesDirty = false
	a.tlCacheDirty = true
	a.mu.Unlock()
}

func (a *App) dayTimelineCached(dayKey string) ([]stats.TimelineBlock, []string) {
	a.mu.Lock()
	if !a.tlCacheDirty && a.cachedTlDay == dayKey {
		b, h := a.cachedTlBlocks, a.cachedTlHidden
		a.mu.Unlock()
		return b, h
	}
	a.mu.Unlock()

	history := a.historyCached()
	ov := a.loadOverrides()
	blocks := stats.LayoutTimelineWithMinSpan(stats.DayTimeline(history, dayKey, &ov), tlMinVisualMin)
	hidden := stats.HiddenTimelineKeys(history, dayKey, &ov)

	a.mu.Lock()
	a.cachedTlDay = dayKey
	a.cachedTlBlocks = blocks
	a.cachedTlHidden = hidden
	a.tlCacheDirty = false
	a.mu.Unlock()
	return blocks, hidden
}

func (a *App) rangeSummaryCached(period string) stats.PeriodSummary {
	today := a.Todo.TodayKey()
	a.mu.Lock()
	if !a.summaryDirty && a.cachedSumPeriod == period && a.cachedSumDay == today {
		s := a.cachedSummary
		a.mu.Unlock()
		return s
	}
	a.mu.Unlock()

	history := a.historyCached()
	days := a.Todo.Days()
	sum := stats.Summary(history, days, period, a.now())

	a.mu.Lock()
	a.cachedSumPeriod = period
	a.cachedSumDay = today
	a.cachedSummary = sum
	a.summaryDirty = false
	a.mu.Unlock()
	return sum
}

func (a *App) yearSummaryCached() stats.YearSummary {
	today := a.Todo.TodayKey()
	a.mu.Lock()
	if !a.yearDirty && a.cachedYearDay == today {
		ys := a.cachedYearSum
		a.mu.Unlock()
		return ys
	}
	a.mu.Unlock()

	history := a.historyCached()
	days := a.Todo.Days()
	ys := stats.YearlySummary(history, days, a.now())

	a.mu.Lock()
	a.cachedYearDay = today
	a.cachedYearSum = ys
	a.yearDirty = false
	a.mu.Unlock()
	return ys
}

func (a *App) renderStatsPanel(c *ui.Context, p Palette) {
	if a.tlDayKey == "" {
		a.tlDayKey = a.Todo.TodayKey()
	}

	var rangeSum *stats.PeriodSummary
	var yearSum *stats.YearSummary
	if a.statsTab == 1 || a.statsTab == 2 {
		period := "week"
		if a.statsTab == 2 {
			period = "month"
		}
		s := a.rangeSummaryCached(period)
		rangeSum = &s
	} else if a.statsTab == 3 {
		ys := a.yearSummaryCached()
		yearSum = &ys
	}

	ui.Column(c).Fill().AlignItems(ui.Center).Background(p.Paper).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(980).Grow(1).MinHeight(0).Padding(PagePadTop, PagePadX, 18, PagePadX).Gap(14).Children(func() {
			// Page header: same title treatment as Today
			ui.Row(c).FillWidth().Height(40).AlignItems(ui.Center).Gap(10).Children(func() {
				plainTitle(c, "专注统计", p)
				if rangeSum != nil {
					ui.Textf(c, "%s 至 %s", rangeSum.Since, rangeSum.Until).
						FontSize(12.5).FontFeatures("tnum").TextColor(p.Ink4).Padding(6, 0, 0, 2)
				} else if yearSum != nil {
					ui.Textf(c, "过去 52 周 · %s 至 %s", yearSum.Since, yearSum.Until).
						FontSize(12.5).FontFeatures("tnum").TextColor(p.Ink4).Padding(6, 0, 0, 2)
				}
				ui.Spacer(c)
				ui.Segmented(c, &a.statsTab, "每日", "本周", "本月", "年度").Label("统计范围")
			})

			switch a.statsTab {
			case 0:
				a.renderDayTimelineTab(c, p)
			case 1, 2:
				if rangeSum != nil {
					a.renderRangeStatsTab(c, *rangeSum, p)
				}
			case 3:
				if yearSum != nil {
					a.renderYearHeatmapTab(c, *yearSum, p)
				}
			}
		})
	})
}

func (a *App) deleteFocusHistory(c *ui.Context, indices []int, label string) {
	removed := a.Timer.DeleteHistoryByIndices(indices)
	if len(removed) == 0 {
		return
	}
	a.lastUndoHistory = removed
	a.tlActionKey = ""
	a.markTotalsDirty()
	a.markStatsDirty()
	msg := fmt.Sprintf("已删除专注记录（%s）", label)
	if len(removed) > 1 {
		msg = fmt.Sprintf("已删除 %d 条专注记录（%s）", len(removed), label)
	}
	c.ToastAction(msg, "撤销", func() {
		if len(a.lastUndoHistory) > 0 {
			a.Timer.RestoreHistoryEntries(a.lastUndoHistory)
			a.lastUndoHistory = nil
			a.markTotalsDirty()
			a.markStatsDirty()
		}
	})
}

func (a *App) renderDayTimelineTab(c *ui.Context, p Palette) {
	blocks, hiddenKeys := a.dayTimelineCached(a.tlDayKey)
	today := a.Todo.TodayKey()

	// Auto-scroll timeline once per day to the first focus block (or ~1h before current time)
	if a.tlScrolledDay != a.tlDayKey {
		a.tlScrolledDay = a.tlDayKey
		targetMin := 8 * 60
		if len(blocks) > 0 {
			targetMin = max(0, blocks[0].StartMin-45)
		} else if a.tlDayKey == today {
			now := a.now().Local()
			targetMin = max(0, now.Hour()*60+now.Minute()-90)
		}
		a.tlScroll.Y = (float32(targetMin) / 60.0) * tlHourHeight
	}

	ui.Column(c).Grow(1).MinHeight(0).Gap(12).Children(func() {
		// Timeline card (mirrors .tl-card)
		ui.Column(c).Grow(1).MinHeight(240).Padding(12, 16, 16, 16).Radius(16).
			Background(p.Paper).Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow).Gap(10).Children(func() {
			// Day navigation bar
			ui.Row(c).FillWidth().Padding(0, 0, 10, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.Line).
				Gap(8).AlignItems(ui.Center).Children(func() {
				prev := iconBtn(c, IconLeft, "时间轴前一天", "前一天", p)
				if prev.Clicked() {
					a.tlDayKey = dateutil.ShiftDay(a.tlDayKey, -1)
					a.tlActionKey = ""
				}
				lbl := dateutil.DayLabel(a.tlDayKey, a.now().Year()) + " " + dateutil.WeekdayLabel(a.tlDayKey)
				if a.tlDayKey == today {
					lbl += " · 今天"
				}
				ui.Text(c, lbl).Font(FontMono).FontSize(13).FontWeight(600).TextColor(p.Ink).Padding(0, 6)
				next := iconBtn(c, IconRight, "时间轴后一天", "后一天", p).Disabled(a.tlDayKey >= today)
				if next.Clicked() {
					a.tlDayKey = dateutil.ShiftDay(a.tlDayKey, 1)
					a.tlActionKey = ""
				}
				if a.tlDayKey != today {
					tb := ui.Button(c, "今天").Height(28).Padding(0, 12).Radius(8).FontSize(12)
					if tb.Clicked() {
						a.tlDayKey = today
						a.tlActionKey = ""
					}
				}
				ui.Spacer(c)
				if len(blocks) == 0 {
					ui.Text(c, "当日暂无时段记录").FontSize(12).TextColor(p.Ink4)
				} else {
					totalM := 0
					for _, b := range blocks {
						totalM += b.Minutes
					}
					ui.Textf(c, "共 %d 段 · %d 分钟", len(blocks), totalM).FontSize(12).FontFeatures("tnum").TextColor(p.Ink3)
				}
			})

			// 24h scrollable schedule
			padTop := float32(12)
			padBot := float32(28)
			totalH := tlHourHeight*24 + padTop + padBot
			ui.Scroll(c.Key("tl-scroll-"+a.tlDayKey)).TrackScroll(&a.tlScroll).Grow(1).MinHeight(0).
				Radius(12).Background(p.TlBg).Border(1, p.BorderSubtle).Children(func() {
				ui.Box(c).FillWidth().Height(totalH).Draw(func(pt *ui.Painter, r ui.Rect) {
					axisX := r.X + 58
					// Night bands: 00-06 and 22-24
					pt.Fill(ui.Rect{X: axisX, Y: r.Y + padTop, W: max(0, r.W-58-12), H: 6 * tlHourHeight}, p.TlNight, 0)
					pt.Fill(ui.Rect{X: axisX, Y: r.Y + padTop + 22*tlHourHeight, W: max(0, r.W-58-12), H: 2 * tlHourHeight}, p.TlNight, 0)
					// Vertical axis hairline
					pt.Line(axisX, r.Y+padTop, axisX, r.Y+padTop+24*tlHourHeight, 1, p.TlGridMajor)
					for h := 0; h < 24; h++ {
						y := r.Y + padTop + float32(h)*tlHourHeight
						col := p.TlGrid
						tickCol := p.Ink4
						if h%6 == 0 {
							col = p.TlGridMajor
							tickCol = p.Ink2
						}
						pt.Line(axisX, y, r.X+r.W-12, y, 1, col)
						pt.Text(r.X+12, y-5, fmt.Sprintf("%02d:00", h), 10.5, tickCol)
					}
					if a.tlDayKey == today {
						now := a.now().Local()
						nowY := r.Y + padTop + (float32(now.Hour()*60+now.Minute())/60.0)*tlHourHeight
						pt.Line(axisX-4, nowY, r.X+r.W-12, nowY, 2, p.TlNow)
						pt.Fill(ui.Rect{X: axisX - 7, Y: nowY - 4, W: 8, H: 8}, p.TlNow, 999)
					}
				}).Children(func() {
					// Axis-safe track container: strictly inside [64px, width-14px], NEVER touches the 00:00 labels!
					ui.Box(c).Absolute().Top(padTop).Bottom(padBot).Left(64).Right(14).Children(func() {
						for idx, b := range blocks {
							top := (float32(b.StartMin) / 60.0) * tlHourHeight
							rawH := (float32(b.EndMin-b.StartMin) / 60.0) * tlHourHeight
							h := max(tlMinBlockH, rawH-tlVertGap)
							cols := max(1, b.Cols)
							colPct := float32(100.0) / float32(cols)
							leftPct := float32(b.Col) * colPct
							bg := p.TlBlockBg
							bd := p.TlBlockBorder
							accent := p.Blue
							ink := p.Ink2
							timeCol := p.BlueDeep
							durLabel := fmt.Sprintf("%d分", b.Minutes)
							if b.Result == "abandoned" {
								bg = p.TlWarnBg
								bd = p.TlWarnBorder
								accent = p.Warn
								ink = p.WarnInk
								timeCol = p.RolledInk
								durLabel = fmt.Sprintf("中断 %d分", b.Minutes)
							}
							isSelected := a.tlActionKey == b.Key
							if isSelected {
								bd = p.Blue
							}

							bCopy := b
							timeRange := fmt.Sprintf("%s–%s", stats.ToHhmm(bCopy.StartMin), stats.ToHhmm(bCopy.EndMin))
							tip := fmt.Sprintf("%s · %s · %s", timeRange, bCopy.Title, durLabel)

							wrap := ui.Box(c.Key(fmt.Sprintf("tlwrap-%d-%s", idx, b.Key))).Absolute().
								Top(top).LeftPercent(leftPct).WidthPercent(colPct).Height(h).
								Padding(0, 4, 0, 0)
							cardHov := wrap.Hovered() || wrap.FocusWithin() || isSelected
							if cardHov {
								wrap.ZIndex(5)
							} else {
								wrap.ZIndex(1)
							}

							wrap.Children(func() {
								card := ui.Row(c.Key(fmt.Sprintf("tlcard-%d-%s", idx, b.Key))).Fill().
									Radius(8).Background(bg).Border(1, bd).Shadow(0, 1, 3, 0, ui.RGBA(0, 46, 88, 0.06)).
									Transition(cardFadeFast).Clip().AlignItems(ui.Stretch)
								if cardHov {
									card.Background(p.Paper).Border(1, p.Blue).Shadow(0, 6, 16, 0, ui.RGBA(0, 46, 88, 0.14))
								}
								card.Children(func() {
									ui.Box(c).Width(3).FillHeight().Background(accent)

									// Clickable content area (sibling of delete button — zero nested interactive buttons!)
									selBtn := ui.ButtonBase(c.Key(fmt.Sprintf("tlblk-%d-%s", idx, bCopy.Key))).
										Grow(1).FillHeight().MinWidth(0).Padding(3, 8).
										Cursor(ui.CursorPointer).Label(bCopy.Title).Tooltip(tip)
									selBtn.Children(func() {
										if cols >= 2 && h >= 44 {
											// Tall multi-column block: stack time + title cleanly on 2 lines
											ui.Column(c).Fill().Justify(ui.Center).Gap(1).Children(func() {
												ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
													ui.Text(c, stats.ToHhmm(bCopy.StartMin)).Font(FontMono).FontSize(10.5).FontWeight(600).TextColor(timeCol).SingleLine()
													ui.Spacer(c)
													if cols == 2 {
														ui.Text(c, durLabel).FontSize(10).FontFeatures("tnum").TextColor(p.Ink3).SingleLine()
													}
												})
												ui.Text(c, bCopy.Title).FontSize(11.5).FontWeight(600).TextColor(ink).SingleLine()
											})
										} else if cols >= 3 {
											// Very narrow compact block: title + start time only
											ui.Row(c).Fill().Gap(4).AlignItems(ui.Center).Children(func() {
												ui.Text(c, bCopy.Title).FontSize(11).FontWeight(600).TextColor(ink).Grow(1).SingleLine()
											})
										} else if cols == 2 {
											// Medium 2-column block: start time + title
											ui.Row(c).Fill().Gap(6).AlignItems(ui.Center).Children(func() {
												ui.Text(c, stats.ToHhmm(bCopy.StartMin)).Font(FontMono).FontSize(10.5).FontWeight(600).TextColor(timeCol).SingleLine()
												ui.Text(c, bCopy.Title).FontSize(11.5).FontWeight(600).TextColor(ink).Grow(1).SingleLine()
											})
										} else {
											// Full-width single-column block: time range + title + duration
											ui.Row(c).Fill().Gap(8).AlignItems(ui.Center).Children(func() {
												ui.Text(c, timeRange).Font(FontMono).FontSize(11).FontWeight(600).TextColor(timeCol).SingleLine()
												ui.Text(c, bCopy.Title).FontSize(12).FontWeight(600).TextColor(ink).Grow(1).SingleLine()
												ui.Text(c, durLabel).FontSize(11).FontFeatures("tnum").TextColor(p.Ink3).SingleLine()
											})
										}
									})
									if selBtn.Clicked() {
										if a.tlActionKey == bCopy.Key {
											a.tlActionKey = ""
										} else {
											a.tlActionKey = bCopy.Key
										}
									}

									if cardHov && (bCopy.Result == "manual" || len(bCopy.HistoryIndices) > 0) {
										ui.Row(c).FillHeight().Padding(0, 5, 0, 0).AlignItems(ui.Center).Children(func() {
											delMini := ui.ButtonBase(c.Key(fmt.Sprintf("tldel-%d-%s", idx, bCopy.Key))).
												Size(20, 20).Radius(5).Center().
												Background(p.DangerSoft).Border(1, p.DangerLine).Transition(colorFadeFast).
												Cursor(ui.CursorPointer).Label("删除时段 " + bCopy.Title).Tooltip("删除这条专注记录")
											if delMini.Hovered() {
												delMini.Border(1, p.Danger)
											}
											delMini.Children(func() {
												ui.Icon(c, IconDelete).Size(11, 11).TextColor(p.Danger)
											})
											if delMini.Clicked() {
												if bCopy.Result == "manual" {
													ov2 := a.loadOverrides()
													filtered := make([]store.ManualBlock, 0, len(ov2.Manual))
													for _, m := range ov2.Manual {
														if m.ID != bCopy.ID {
															filtered = append(filtered, m)
														}
													}
													ov2.Manual = filtered
													a.saveOverrides(ov2)
													a.tlActionKey = ""
													c.Toast("已删除补录时段")
												} else if len(bCopy.HistoryIndices) > 0 {
													a.deleteFocusHistory(c, bCopy.HistoryIndices, bCopy.Title)
												}
											}
										})
									}
								})
							})
						}
					})
				})
			})
		})

		// Selected block action strip
		if a.tlActionKey != "" {
			var sel *stats.TimelineBlock
			for i := range blocks {
				if blocks[i].Key == a.tlActionKey {
					sel = &blocks[i]
					break
				}
			}
			if sel != nil {
				ui.Row(c).Padding(8, 12).Radius(10).Background(p.Surface).Border(1, p.Line).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Textf(c, "已选：%s (%s–%s)", sel.Title, stats.ToHhmm(sel.StartMin), stats.ToHhmm(sel.EndMin)).
						FontSize(12.5).FontWeight(600).TextColor(p.Navy).Grow(1).SingleLine()
					if sel.Result != "manual" {
						edB := ui.Button(c, "改起止").Padding(4, 10).FontSize(12)
						if edB.Clicked() {
							a.tlFormOpen = true
							a.tlFormMode = "edit"
							a.tlEditKey = sel.Key
							a.tlFormTitle = sel.Title
							a.tlFormStart = stats.ToHhmm(sel.StartMin)
							a.tlFormEnd = stats.ToHhmm(sel.EndMin)
						}
						hdB := ui.Button(c, "隐藏").Padding(4, 10).FontSize(12)
						if hdB.Clicked() {
							ov2 := a.loadOverrides()
							hFlag := true
							updated := false
							for i := range ov2.Edits {
								if ov2.Edits[i].Key == sel.Key {
									ov2.Edits[i].Hidden = &hFlag
									updated = true
									break
								}
							}
							if !updated {
								ov2.Edits = append(ov2.Edits, store.TimelineEdit{Key: sel.Key, Hidden: &hFlag})
							}
							a.saveOverrides(ov2)
							a.tlActionKey = ""
						}
						if len(sel.HistoryIndices) > 0 {
							delRec := ui.Button(c, "删除记录").Padding(4, 10).FontSize(12).TextColor(p.Danger)
							if delRec.Clicked() {
								a.deleteFocusHistory(c, sel.HistoryIndices, sel.Title)
							}
						}
					} else {
						delM := ui.Button(c, "删除补录").Padding(4, 10).FontSize(12)
						if delM.Clicked() {
							ov2 := a.loadOverrides()
							filtered := make([]store.ManualBlock, 0, len(ov2.Manual))
							for _, m := range ov2.Manual {
								if m.ID != sel.ID {
									filtered = append(filtered, m)
								}
							}
							ov2.Manual = filtered
							a.saveOverrides(ov2)
							a.tlActionKey = ""
						}
					}
					if sel.TaskID != "" {
						cpTask := ui.Button(c, "复制任务记录").Padding(4, 10).FontSize(12)
						if cpTask.Clicked() {
							td := sel.TaskDay
							if td == "" {
								td = a.tlDayKey
							}
							res := notes.BuildNote(a.Store, notes.BuildRequest{Kind: "task", TaskID: sel.TaskID, TaskDay: td}, a.now())
							if res.OK {
								c.WriteClipboard(res.Text)
								c.Toast("已复制单任务记录")
							} else {
								c.Toast(res.Error)
							}
						}
					}
				})
			}
		}

		// Manual add / edit form
		if a.tlFormOpen {
			ui.Row(c).Padding(10, 12).Radius(12).Background(p.Paper).Border(1, p.Line).
				Shadow(0, 4, 14, 0, ui.RGBA(0, 46, 88, 0.06)).Gap(8).AlignItems(ui.Center).Children(func() {
				if a.tlFormMode == "add" {
					ui.TextInput(c.Key("tl-title"), &a.tlFormTitle).Placeholder("做了什么（必填）").Grow(1).Label("时段名称")
				} else {
					ui.Text(c, "调整时段："+a.tlFormTitle).FontSize(12.5).FontWeight(600).TextColor(p.Navy).Grow(1).SingleLine()
				}
				ui.TextInput(c.Key("tl-st"), &a.tlFormStart).Font(FontMono).Width(74).Placeholder("09:00").Label("开始时间")
				ui.Text(c, "–").TextColor(p.Ink3)
				ui.TextInput(c.Key("tl-en"), &a.tlFormEnd).Font(FontMono).Width(74).Placeholder("10:00").Label("结束时间")
				saveB := primaryBtn(c, "保存", 12.5, p).Padding(5, 14)
				canB := ui.Button(c, "取消").Padding(5, 14).FontSize(12.5)
				if saveB.Clicked() {
					stNorm, ok1 := dateutil.NormalizeAlarm(a.tlFormStart)
					enNorm, ok2 := dateutil.NormalizeAlarm(a.tlFormEnd)
					if !ok1 || !ok2 || enNorm <= stNorm {
						c.Toast("请输入合法起止时刻且结束晚于开始")
					} else if a.tlFormMode == "add" {
						ttl := strings.TrimSpace(a.tlFormTitle)
						if ttl == "" {
							c.Toast("请填写时段名称")
						} else {
							ov2 := a.loadOverrides()
							ov2.Manual = append(ov2.Manual, store.ManualBlock{
								ID: store.RandID("m"), Day: a.tlDayKey, Start: stNorm, End: enNorm, Title: ttl,
							})
							a.saveOverrides(ov2)
							a.tlFormOpen = false
						}
					} else {
						ov2 := a.loadOverrides()
						updated := false
						for i := range ov2.Edits {
							if ov2.Edits[i].Key == a.tlEditKey {
								ov2.Edits[i].Start = stNorm
								ov2.Edits[i].End = enNorm
								updated = true
								break
							}
						}
						if !updated {
							ov2.Edits = append(ov2.Edits, store.TimelineEdit{Key: a.tlEditKey, Start: stNorm, End: enNorm})
						}
						a.saveOverrides(ov2)
						a.tlFormOpen = false
					}
				} else if canB.Clicked() {
					a.tlFormOpen = false
				}
			})
		}

		// Collect unbound focus indices for 1-click day cleanup
		var dayUnboundIdx []int
		for _, b := range blocks {
			if b.TaskID == "" && b.Result != "manual" && len(b.HistoryIndices) > 0 {
				dayUnboundIdx = append(dayUnboundIdx, b.HistoryIndices...)
			}
		}

		// Bottom action bar (mirrors .tl-actions)
		ui.Row(c).FillWidth().Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.Line).
			Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			addB := primaryBtn(c, "+ 补录时段", 12.5, p).Height(34).Padding(0, 14).Radius(9).
				Shadow(0, 4, 12, 0, p.Blue.Alpha(0.22))
			if addB.Clicked() {
				a.tlFormOpen = true
				a.tlFormMode = "add"
				a.tlFormTitle = ""
				a.tlFormStart = "09:00"
				a.tlFormEnd = "09:30"
			}
			if len(dayUnboundIdx) > 0 {
				clrUb := ui.ButtonBase(c).Height(34).Padding(0, 12).Radius(9).
					Background(p.DangerSoft).Border(1, p.DangerLine).Transition(colorFadeFast).
					Cursor(ui.CursorPointer).Label("清空当日未定任务").Tooltip("删除当日所有未定任务的专注记录（可撤销）")
				if clrUb.Hovered() {
					clrUb.Border(1, p.Danger)
				}
				clrUb.Children(func() {
					ui.Textf(c, "清空未定 (%d)", len(dayUnboundIdx)).FontSize(12).FontWeight(600).TextColor(p.Danger).SingleLine()
				})
				if clrUb.Clicked() {
					a.deleteFocusHistory(c, dayUnboundIdx, "当日全部未定任务")
				}
			}
			if len(hiddenKeys) > 0 {
				restB := ui.Button(c, fmt.Sprintf("恢复隐藏 (%d)", len(hiddenKeys))).Height(34).Padding(0, 12).Radius(9).FontSize(12)
				if restB.Clicked() {
					restoreSet := map[string]bool{}
					for _, k := range hiddenKeys {
						restoreSet[k] = true
					}
					ov2 := a.loadOverrides()
					kept := make([]store.TimelineEdit, 0, len(ov2.Edits))
					for _, ed := range ov2.Edits {
						if !restoreSet[ed.Key] {
							kept = append(kept, ed)
							continue
						}
						ed.Hidden = nil
						if ed.Start != "" || ed.End != "" {
							kept = append(kept, ed)
						}
					}
					ov2.Edits = kept
					a.saveOverrides(ov2)
					c.Toast(fmt.Sprintf("已恢复当日隐藏的 %d 个时段", len(hiddenKeys)))
				}
			}
			ui.Spacer(c)
			dailyLabel := "复制今日复盘"
			if a.tlDayKey != today {
				dailyLabel = "复制当日复盘"
			}
			copyDaily := ui.Button(c, dailyLabel).Height(34).Padding(0, 12).Radius(9).FontSize(12)
			if copyDaily.Clicked() {
				res := notes.BuildNote(a.Store, notes.BuildRequest{Kind: "daily", Day: a.tlDayKey}, a.now())
				if res.OK {
					c.WriteClipboard(res.Text)
					c.Toast("已复制「" + res.Name + "」到剪贴板")
				}
			}
			saveDaily := ui.Button(c, "存为 .md").Height(34).Padding(0, 12).Radius(9).FontSize(12)
			if saveDaily.Clicked() {
				a.SaveNoteToFile(notes.BuildRequest{Kind: "daily", Day: a.tlDayKey})
			}
			copyWeekly := ui.Button(c, "复制周报").Height(34).Padding(0, 12).Radius(9).FontSize(12)
			if copyWeekly.Clicked() {
				res := notes.BuildNote(a.Store, notes.BuildRequest{Kind: "weekly"}, a.now())
				if res.OK {
					c.WriteClipboard(res.Text)
					c.Toast("已复制「" + res.Name + "」到剪贴板")
				}
			}
		})
	})
}

func (a *App) renderRangeStatsTab(c *ui.Context, sum stats.PeriodSummary, p Palette) {
	ui.Scroll(c.Key("stats-range-scroll")).TrackScroll(&a.statsScroll).Grow(1).Gap(18).Children(func() {
		// 4 KPI Cards (mirrors .stats-grid with 3px left accent stripe + FontNum rolling display numerals)
		kpiRoll := ui.ContentTransition{Numeric: true}
		ui.Grid(c).Columns(4).Gap(12).FillWidth().Children(func() {
			hero := ui.Box(c).Radius(14).Height(92).
				LinearGradient(ui.LinearGradient{From: p.Paper, To: p.StatHeroB, Angle: 165, Oklab: true}).
				Border(1, p.AccentVeil).Shadow(0, 4, 18, 0, p.CardShadow).Clip()
			hero.Children(func() {
				ribbonBox := ui.Box(c).Absolute().Bottom(0).Left(3).Right(0).Height(26).ZIndex(0).PassThrough()
				a.miniSea.DrawTideRibbon(c, ribbonBox, SeaView{
					RemainRatio:  1.0,
					Idle:         true,
					Motion:       "off",
					Dark:         p.Dark,
					ReduceMotion: true,
					Now:          a.now(),
				})
				ui.Row(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(1).Children(func() {
					ui.Box(c).Width(3).FillHeight().Background(p.Blue)
					ui.Column(c).Padding(14, 16).Gap(4).Children(func() {
						ui.Textf(c, "%d", sum.Minutes).Font(FontNum).FontSize(34).FontWeight(700).
							FontFeatures("tnum").TextColor(p.BlueDeep).ContentTransition(kpiRoll)
						ui.Text(c, "专注分钟").FontSize(12).FontWeight(600).TextColor(p.Ink2)
					})
				})
			})
			for _, item := range []struct {
				val   int
				label string
			}{
				{sum.Rounds, "完成轮次"},
				{sum.Tasks, "完成任务"},
				{sum.Streak, "连续天数"},
			} {
				card := ui.Row(c).Radius(14).Background(p.Paper).Border(1, p.Line).
					Shadow(0, 4, 18, 0, p.CardShadow).Clip()
				card.Children(func() {
					ui.Box(c).Width(3).FillHeight().Background(p.AccentVeil)
					ui.Column(c).Padding(16, 16).Gap(6).Children(func() {
						ui.Textf(c, "%d", item.val).Font(FontNum).FontSize(30).FontWeight(700).
							FontFeatures("tnum").TextColor(p.Navy).ContentTransition(kpiRoll)
						ui.Text(c, item.label).FontSize(12).TextColor(p.Ink3)
					})
				})
			}
		})

		// Section heading with hairline divider + peak scale (mirrors .stats-section)
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "专注时段").FontSize(14).FontWeight(700).TextColor(p.Navy).SingleLine()
			ui.Box(c).Grow(1).Height(1).Background(p.Line)
			if sum.Peak > 0 && sum.PeakIndex >= 0 {
				ui.Textf(c, "峰值 %02d–%02d 点 · %d 分钟", sum.PeakIndex*2, sum.PeakIndex*2+2, sum.Peak).
					Font(FontMono).FontSize(11.5).FontWeight(600).TextColor(p.Ink3).SingleLine()
			}
		})

		// 12-bin chart card (mirrors .hours with smooth animated bars & centered labels)
		chartCard := ui.Column(c.Key("stats-hours-card")).FillWidth().Padding(20, 14, 16, 14).Radius(14).
			LinearGradient(ui.LinearGradient{From: p.ChartBgA, To: p.ChartBgB, Angle: 180}).
			Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow)

		// Animate normalized bar heights (0..1) so switching week/month glides smoothly
		maxVal := max(1, sum.Peak)
		var animFrac [12]float32
		for i := 0; i < 12; i++ {
			target := float32(0)
			if sum.Bins[i] > 0 {
				target = float32(math.Min(1, float64(sum.Bins[i])/float64(maxVal)))
			}
			animFrac[i] = chartCard.AnimateTo(fmt.Sprintf("bar-%d", i), target, ui.Snappy)
		}

		chartCard.Children(func() {
			ui.Box(c).FillWidth().Height(180).Draw(func(pt *ui.Painter, r ui.Rect) {
				topPad := float32(22)
				botPad := float32(26)
				chartH := r.H - topPad - botPad
				baseY := r.Y + topPad + chartH

				// Subtle horizontal 25% grid lines
				for g := 0; g <= 4; g++ {
					y := r.Y + topPad + chartH*float32(g)/4.0
					pt.Line(r.X, y, r.X+r.W, y, 1, p.ChartGrid)
				}
				slotW := r.W / 12.0
				barW := min(34, slotW*0.58)
				for i := 0; i < 12; i++ {
					cx := r.X + slotW*float32(i) + (slotW-barW)/2
					// Slot backing rail
					pt.Fill(ui.Rect{X: cx, Y: r.Y + topPad, W: barW, H: chartH}, p.ChartSlot, 6)
					val := sum.Bins[i]
					frac := animFrac[i]
					if frac > 0.005 {
						bh := max(6, chartH*frac)
						by := baseY - bh
						pt.FillGradient(ui.Rect{X: cx, Y: by, W: barW, H: bh},
							ui.LinearGradient{From: p.Blue2, To: p.Blue, Angle: 180}, 6)
						if val > 0 {
							lbl := fmt.Sprint(val)
							lw, _ := pt.MeasureText(80, ui.Span{Text: lbl, Size: 10.5, Weight: 600})
							pt.Text(cx+(barW-lw)/2, by-16, lbl, 10.5, p.BlueDeep)
						}
					}
					hr := fmt.Sprintf("%02d", i*2)
					hw, _ := pt.MeasureText(40, ui.Span{Text: hr, Size: 11})
					pt.Text(cx+(barW-hw)/2, baseY+8, hr, 11, p.Ink4)
				}
			})
		})

		// Focus Records Detail List (cached history, quick "仅未定任务" filter, batch cleanup & 1-click delete with Undo!)
		history := a.historyCached()
		type recItem struct {
			idx   int
			entry store.HistoryEntry
		}
		var matched []recItem
		var unboundIndices []int
		for i := len(history) - 1; i >= 0; i-- {
			h := history[i]
			if h.Phase != "focus" || h.Day < sum.Since || h.Day > sum.Until {
				continue
			}
			isUnbound := h.TaskID == ""
			if isUnbound {
				unboundIndices = append(unboundIndices, i)
			}
			if a.statsOnlyUnbound && !isUnbound {
				continue
			}
			matched = append(matched, recItem{idx: i, entry: h})
		}
		unboundCount := len(unboundIndices)

		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "专注记录明细（%d 条）", len(matched)).FontSize(14).FontWeight(700).TextColor(p.Navy).SingleLine()
			ui.Box(c).Grow(1).Height(1).Background(p.Line)
			if unboundCount > 0 {
				clrAll := ui.ButtonBase(c).Padding(4, 10).Radius(999).Background(p.DangerSoft).Border(1, p.DangerLine).
					Transition(colorFadeFast).Cursor(ui.CursorPointer).Label("清空未定任务记录").Tooltip("一键删除当前统计范围内全部未定任务的专注记录（可撤销）")
				if clrAll.Hovered() {
					clrAll.Border(1, p.Danger)
				}
				clrAll.Children(func() {
					ui.Textf(c, "清空未定 (%d)", unboundCount).FontSize(11.5).FontWeight(600).TextColor(p.Danger).SingleLine()
				})
				if clrAll.Clicked() {
					a.deleteFocusHistory(c, unboundIndices, "范围内全部未定任务")
				}
			}
			ubLabel := fmt.Sprintf("仅未定任务 (%d)", unboundCount)
			ubBtn := ui.ButtonBase(c).Padding(4, 10).Radius(999).Background(p.Paper).Border(1, p.Line).
				Transition(colorFadeFast).Cursor(ui.CursorPointer).Label("筛选未定任务").Checked(a.statsOnlyUnbound)
			if a.statsOnlyUnbound {
				ubBtn.Background(p.AccentSoft).Border(1, p.Blue)
			} else if ubBtn.Hovered() {
				ubBtn.Border(1, p.Blue)
			}
			ubBtn.Children(func() {
				col := p.Ink3
				if a.statsOnlyUnbound {
					col = p.BlueDeep
				}
				ui.Text(c, ubLabel).FontSize(11.5).FontWeight(600).TextColor(col).SingleLine()
			})
			if ubBtn.Clicked() {
				a.statsOnlyUnbound = !a.statsOnlyUnbound
			}
		})

		displayList := matched
		const maxInitialRecs = 60
		truncated := !a.statsShowAllRecs && len(matched) > maxInitialRecs
		if truncated {
			displayList = matched[:maxInitialRecs]
		}

		ui.Column(c).FillWidth().Padding(8, 12).Radius(14).Background(p.Paper).Border(1, p.Line).
			Shadow(0, 4, 18, 0, p.CardShadow).Gap(2).Children(func() {
			if len(matched) == 0 {
				ui.Row(c).FillWidth().Padding(18, 10).Center().Children(func() {
					msg := "该时间范围内暂无专注记录"
					if a.statsOnlyUnbound {
						msg = "该时间范围内没有未定任务的专注记录"
					}
					ui.Text(c, msg).FontSize(12.5).TextColor(p.Ink4)
				})
			}
			for _, it := range displayList {
				h := it.entry
				origIdx := it.idx
				title := h.TaskTitle
				if title == "" {
					if h.TaskID != "" {
						title = "（未命名任务）"
					} else {
						title = "未定任务的专注"
					}
				}
				timeTag := h.Day
				if st, err := time.Parse(time.RFC3339Nano, h.StartedAt); err == nil {
					timeTag += " " + st.Local().Format("15:04")
				} else if fin, err := time.Parse(time.RFC3339Nano, h.FinishedAt); err == nil {
					timeTag += " " + fin.Local().Format("15:04")
				}
				row := ui.Row(c.Key(fmt.Sprintf("hist-rec-%d", origIdx))).FillWidth().Padding(9, 8).Radius(8).
					BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Transition(colorFadeFast).Gap(10).AlignItems(ui.Center)
				if row.Hovered() {
					row.Background(p.AccentSoft)
				}
				row.Children(func() {
					ui.Text(c, timeTag).Font(FontMono).FontSize(11.5).TextColor(p.Ink3).Width(116).SingleLine()
					ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(p.Ink).Grow(1).SingleLine()
					if h.TaskID == "" {
						badgeChip(c, "未定任务", p.Surface, p.Ink3, p.LineSoft)
					}
					if h.Result == "abandoned" {
						badgeChip(c, fmt.Sprintf("中断 %d分", h.ActualMinutes), p.WarnBg, p.WarnInk, p.WarnLine)
					} else {
						badgeChip(c, fmt.Sprintf("%d 分钟", h.PlannedMinutes), p.AccentSoft, p.BlueDeep, ui.Transparent)
					}
					// Same quiet delete as a task row on Today: an icon that only tints on hover.
					delB := iconBtn(c.Key(fmt.Sprintf("hist-del-%d", origIdx)), IconDelete,
						fmt.Sprintf("删除专注 %s %d", title, origIdx), "删除此条专注记录（可撤销）", p).Size(28, 28)
					if delB.Clicked() {
						a.deleteFocusHistory(c, []int{origIdx}, title)
					}
				})
			}
			if truncated {
				ui.Row(c).FillWidth().Padding(8, 0, 4, 0).Center().Children(func() {
					moreB := ui.Button(c, fmt.Sprintf("展开剩余 %d 条记录", len(matched)-maxInitialRecs)).Height(30).Padding(0, 14).Radius(8).FontSize(12)
					if moreB.Clicked() {
						a.statsShowAllRecs = true
					}
				})
			}
		})

		ui.Row(c).Padding(12, 0, 0, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.Line).Children(func() {
			ui.Text(c, "说明：上方 KPI 与时段分布统计已完成的专注轮次；在明细中删除记录会同步更新统计并支持底部一键撤销。").
				FontSize(12).TextColor(p.Ink4)
		})
	})
}

func (a *App) renderYearHeatmapTab(c *ui.Context, ys stats.YearSummary, p Palette) {
	ui.Scroll(c.Key("stats-year-scroll")).Grow(1).Gap(16).Children(func() {
		// 4 Annual KPI Cards
		kpiRoll := ui.ContentTransition{Numeric: true}
		ui.Grid(c).Columns(4).Gap(12).FillWidth().Children(func() {
			hero := ui.Box(c).Radius(14).Height(92).
				LinearGradient(ui.LinearGradient{From: p.Paper, To: p.StatHeroB, Angle: 165, Oklab: true}).
				Border(1, p.AccentVeil).Shadow(0, 4, 18, 0, p.CardShadow).Clip()
			hero.Children(func() {
				ribbonBox := ui.Box(c).Absolute().Bottom(0).Left(3).Right(0).Height(26).ZIndex(0).PassThrough()
				a.miniSea.DrawTideRibbon(c, ribbonBox, SeaView{
					RemainRatio:  1.0,
					Idle:         true,
					Motion:       "off",
					Dark:         p.Dark,
					ReduceMotion: true,
					Now:          a.now(),
				})
				ui.Row(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(1).Children(func() {
					ui.Box(c).Width(3).FillHeight().Background(p.Blue)
					ui.Column(c).Padding(14, 16).Gap(4).Children(func() {
						ui.Textf(c, "%d", ys.TotalMinutes).Font(FontNum).FontSize(34).FontWeight(700).
							FontFeatures("tnum").TextColor(p.BlueDeep).ContentTransition(kpiRoll)
						ui.Text(c, "年度专注分钟").FontSize(12).FontWeight(600).TextColor(p.Ink2)
					})
				})
			})
			for _, item := range []struct {
				val   int
				label string
			}{
				{ys.ActiveDays, "活跃天数"},
				{ys.LongestStreak, "最长连胜"},
				{ys.TotalRounds, "完成轮次"},
			} {
				card := ui.Row(c).Radius(14).Background(p.Paper).Border(1, p.Line).
					Shadow(0, 4, 18, 0, p.CardShadow).Clip()
				card.Children(func() {
					ui.Box(c).Width(3).FillHeight().Background(p.AccentVeil)
					ui.Column(c).Padding(16, 16).Gap(6).Children(func() {
						ui.Textf(c, "%d", item.val).Font(FontNum).FontSize(30).FontWeight(700).
							FontFeatures("tnum").TextColor(p.Navy).ContentTransition(kpiRoll)
						ui.Text(c, item.label).FontSize(12).TextColor(p.Ink3).SingleLine()
					})
				})
			}
		})

		// 52-Week Pure Ocean-Blue Heatmap Card
		levelCols := [5]ui.Color{
			ui.Hex("#EAF2FA"),
			ui.Hex("#BFE0FF"),
			ui.Hex("#6FB8FF"),
			ui.Hex("#1A88FF"),
			ui.Hex("#0057B8"),
		}
		if p.Dark {
			levelCols = [5]ui.Color{
				ui.Hex("#192638"),
				ui.Hex("#1D3D61"),
				ui.Hex("#245C96"),
				ui.Hex("#2F7FD4"),
				ui.Hex("#5EB2FF"),
			}
		}

		hmCard := ui.Column(c.Key("year-heatmap-card")).FillWidth().Padding(18, 20).Radius(16).
			Background(p.Paper).Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow).Gap(12)
		hmCard.Children(func() {
			hoverText := "悬停查看当日记录，点击方块在下方固定显示"

			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "年度热力图").FontSize(14).FontWeight(700).TextColor(p.Navy)
				ui.Textf(c, "完成 %d 项任务", ys.TotalTasks).FontSize(12).TextColor(p.Ink3)
				ui.Spacer(c)
				ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "少").FontSize(10.5).TextColor(p.Ink4)
					for lv := 0; lv < 5; lv++ {
						ui.Box(c).Size(10, 10).Radius(2.5).Background(levelCols[lv])
					}
					ui.Text(c, "多").FontSize(10.5).TextColor(p.Ink4)
				})
			})

			hmBox := ui.Box(c.Key("year-hm-canvas")).FillWidth().Height(156)
			hpx, hpy, hpIn := hmBox.PointerPosition()
			if hmBox.Clicked() && a.yearHoverDay != "" {
				a.yearSelDay = a.yearHoverDay
			}
			selDay := a.yearSelDay
			hmBox.Cursor(ui.CursorPointer).Draw(func(pt *ui.Painter, r ui.Rect) {
				leftPad := float32(24)
				topPad := float32(20)
				botPad := float32(22)
				availW := r.W - leftPad - 6
				availH := r.H - topPad - botPad
				if availW <= 40 || availH <= 40 {
					return
				}
				cols := 52
				gap := float32(3)
				cellW := (availW - float32(cols-1)*gap) / float32(cols)
				cellH := (availH - float32(6)*gap) / 7.0
				cell := min(cellW, cellH)
				if cell < 4 {
					cell = 4
				}
				gridW := float32(cols)*cell + float32(cols-1)*gap
				startX := r.X + leftPad + max(0, (availW-gridW)/2)
				startY := r.Y + topPad

				for _, wd := range []struct {
					row int
					lbl string
				}{{0, "一"}, {2, "三"}, {4, "五"}} {
					pt.Text(startX-18, startY+float32(wd.row)*(cell+gap)+cell*0.1, wd.lbl, 10, p.Ink4)
				}

				lastMonth := ""
				var tipDay *stats.YearHeatmapDay
				for i := range ys.Days {
					d := &ys.Days[i]
					col := i / 7
					row := i % 7
					if col >= cols {
						continue
					}
					cx := startX + float32(col)*(cell+gap)
					cy := startY + float32(row)*(cell+gap)
					if row == 0 && len(d.Day) >= 7 {
						mo := d.Day[5:7]
						if mo != lastMonth {
							lastMonth = mo
							pt.Text(cx, r.Y+4, strings.TrimLeft(mo, "0")+"月", 10, p.Ink4)
						}
					}
					cellRect := ui.Rect{X: cx, Y: cy, W: cell, H: cell}
					pt.Fill(cellRect, levelCols[d.Level], 2.5)
					if d.Day == selDay {
						pt.Stroke(cellRect, p.Blue, 2.5, 2)
					}
					if hpIn && hpx >= cx-1 && hpx <= cx+cell+1 && hpy >= cy-1 && hpy <= cy+cell+1 {
						pt.Stroke(cellRect, p.Navy, 2.5, 1.5)
						tipDay = d
					}
				}
				// Remembered for the next build so a click can pin the hovered day.
				if tipDay != nil {
					a.yearHoverDay = tipDay.Day
				} else {
					a.yearHoverDay = ""
				}
				if tipDay != nil {
					txt := fmt.Sprintf("%s · %d 分钟（%d 轮）", tipDay.Day, tipDay.Minutes, tipDay.Rounds)
					if tipDay.Minutes == 0 {
						txt = fmt.Sprintf("%s · 无记录", tipDay.Day)
					}
					pt.Text(startX, startY+7*(cell+gap)+6, txt, 11.5, p.BlueDeep)
				} else {
					pt.Text(startX, startY+7*(cell+gap)+6, hoverText, 11, p.Ink4)
				}
			})
		})

		// Below the heatmap: month-by-month totals beside the pinned day's detail
		ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Stretch).Children(func() {
			a.renderYearMonthBars(c, ys, p)
			a.renderYearDayDetail(c, ys, p)
		})
	})
}

// renderYearMonthBars draws focus minutes per calendar month across the heatmap's 52-week window.
func (a *App) renderYearMonthBars(c *ui.Context, ys stats.YearSummary, p Palette) {
	type monthSum struct {
		key     string // YYYY-MM
		minutes int
	}
	var months []monthSum
	for _, d := range ys.Days {
		if len(d.Day) < 7 {
			continue
		}
		k := d.Day[:7]
		if n := len(months); n == 0 || months[n-1].key != k {
			months = append(months, monthSum{key: k})
		}
		months[len(months)-1].minutes += d.Minutes
	}
	maxMin, best := 0, -1
	for i, m := range months {
		if m.minutes > maxMin {
			maxMin, best = m.minutes, i
		}
	}

	ui.Column(c.Key("year-month-card")).Grow(1).MinWidth(0).Padding(18, 20).Radius(16).
		Background(p.Paper).Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow).Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "月度专注").FontSize(14).FontWeight(700).TextColor(p.Navy)
			if best >= 0 {
				ui.Textf(c, "最高 %s月 · %d 分钟", strings.TrimLeft(months[best].key[5:], "0"), maxMin).
					FontSize(12).TextColor(p.Ink3).SingleLine()
			} else {
				ui.Text(c, "还没有专注记录").FontSize(12).TextColor(p.Ink3)
			}
		})
		ui.Box(c.Key("year-month-canvas")).FillWidth().Height(120).Draw(func(pt *ui.Painter, r ui.Rect) {
			n := len(months)
			if n == 0 || r.W < 60 {
				return
			}
			labelH := float32(18)
			gap := float32(6)
			barW := (r.W - float32(n-1)*gap) / float32(n)
			chartH := r.H - labelH
			for i, m := range months {
				x := r.X + float32(i)*(barW+gap)
				// Empty months keep a faint baseline so the axis still reads as twelve steps.
				h := float32(2)
				col := p.LineSoft
				if m.minutes > 0 && maxMin > 0 {
					h = max(4, chartH*float32(m.minutes)/float32(maxMin))
					col = p.BlueSoft
					if i == best {
						col = p.Blue
					}
				}
				pt.Fill(ui.Rect{X: x, Y: r.Y + chartH - h, W: barW, H: h}, col, 3)
				pt.Text(x+barW/2-6, r.Y+chartH+4, strings.TrimLeft(m.key[5:], "0"), 10, p.Ink4)
			}
		})
	})
}

// renderYearDayDetail shows the day pinned by clicking a heatmap cell (today until one is picked).
func (a *App) renderYearDayDetail(c *ui.Context, ys stats.YearSummary, p Palette) {
	day := a.yearSelDay
	if day == "" {
		day = a.Todo.TodayKey()
	}
	minutes, rounds := 0, 0
	for _, d := range ys.Days {
		if d.Day == day {
			minutes, rounds = d.Minutes, d.Rounds
			break
		}
	}
	doneTasks, totalTasks := 0, 0
	for _, tk := range a.Todo.TasksForDay(day) {
		totalTasks++
		if tk.Done {
			doneTasks++
		}
	}

	ui.Column(c.Key("year-day-card")).Width(250).Padding(18, 20).Radius(16).
		Background(p.Paper).Border(1, p.Line).Shadow(0, 4, 18, 0, p.CardShadow).Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "当日详情").FontSize(14).FontWeight(700).TextColor(p.Navy)
			ui.Text(c, day).Font(FontMono).FontSize(11.5).TextColor(p.Ink3).SingleLine()
		})
		ui.Row(c).Gap(6).AlignItems(ui.End).Children(func() {
			ui.Textf(c, "%d", minutes).Font(FontNum).FontSize(30).FontWeight(700).FontFeatures("tnum").TextColor(p.BlueDeep)
			ui.Text(c, "分钟").FontSize(12).TextColor(p.Ink3).Padding(0, 0, 5, 0)
		})
		ui.Textf(c, "%d 轮专注 · 完成 %d/%d 项任务", rounds, doneTasks, totalTasks).FontSize(12.5).TextColor(p.Ink2)
		ui.Spacer(c)
		open := ui.Button(c, "查看当日时间轴").FillWidth().Height(32).Radius(9).FontSize(12.5)
		if open.Clicked() {
			a.tlDayKey = day
			a.tlScrolledDay = ""
			a.statsTab = 0
			a.markStatsDirty()
		}
	})
}
