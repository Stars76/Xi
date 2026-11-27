package uiapp

import (
	"fmt"

	"github.com/Stars76/526Hz/internal/notes"
	"github.com/Stars76/526Hz/internal/store"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *App) renderSettingsPanel(c *ui.Context, snap timer.Snapshot, p Palette) {
	ui.Column(c).Fill().AlignItems(ui.Center).Background(p.Paper).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(780).Grow(1).MinHeight(0).Padding(PagePadTop, PagePadX, 18, PagePadX).Gap(14).Children(func() {
			ui.Row(c).FillWidth().Height(40).AlignItems(ui.Center).Gap(10).Children(func() {
				pageTitle(c, "设", "置", p)
				ui.Text(c, "与备份").FontSize(12.5).TextColor(p.Ink4).Padding(6, 0, 0, 2)
			})

			ui.Scroll(c.Key("settings-scroll")).TrackScroll(&a.settingsScrl).Grow(1).MinHeight(0).Padding(4, 6).Gap(18).Children(func() {
			// 1) Theme
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(14).Children(func() {
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Text(c, "外观主题").FontSize(16).FontWeight(700).TextColor(p.Navy)
					ui.Text(c, "浅色、深色或跟随系统，主窗与专注钟小窗同步生效。").FontSize(13).TextColor(p.Ink3)
				})
				themeIdx := 0
				switch snap.Prefs.Theme {
				case "dark":
					themeIdx = 1
				case "system":
					themeIdx = 2
				}
				if ui.Segmented(c, &themeIdx, "浅色", "深色", "跟随系统").Label("外观主题").Changed() {
					modes := []string{"light", "dark", "system"}
					m := modes[themeIdx]
					a.Timer.SetPrefs(timer.PrefsPatch{Theme: &m})
				}
			})

			if AutostartSupported() {
				if !a.autostartLoaded {
					a.autostartOn = IsAutostartEnabled()
					a.autostartLoaded = true
				}
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(14).Children(func() {
					ui.Column(c).Grow(1).Gap(3).Children(func() {
						ui.Text(c, "开机自启").FontSize(16).FontWeight(700).TextColor(p.Navy)
						ui.Text(c, "开机后自动静默启动并常驻系统托盘，随时开始专注。").FontSize(13).TextColor(p.Ink3)
					})
					autoOn := a.autostartOn
					if ui.Switch(c.Key("settings-autostart"), &autoOn).Label("开机自启").Changed() {
						if err := SetAutostartEnabled(autoOn); err != nil {
							c.Toast("开机自启设置失败：" + err.Error())
						} else {
							a.autostartOn = autoOn
							if autoOn {
								c.Toast("已开启开机静默自启")
							} else {
								c.Toast("已关闭开机自启")
							}
						}
					}
				})
			}

			ui.Divider(c)

			// Focus clock & reminders: the same preferences as the Focus Clock's quick overlay
			ui.Column(c).FillWidth().Gap(10).Children(func() {
				ui.Text(c, "专注与提醒").FontSize(16).FontWeight(700).TextColor(p.Navy)
				ui.Text(c, "提示音、系统通知、动画与托盘行为；专注钟右上角的快捷设置与这里同步。").FontSize(13).TextColor(p.Ink3)
				a.renderTimerPrefRows(c, snap, p, 14, false)
			})

			ui.Divider(c)

			// Keyboard shortcuts reference
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "快捷键").FontSize(16).FontWeight(700).TextColor(p.Navy)
				for _, sc := range [][2]string{
					{"空格", "开始 / 暂停 / 继续专注"},
					{"Ctrl+N", "回到今日事并新建任务"},
					{"Ctrl+1 … Ctrl+5", "切换 今日事 / 专注钟 / 日历 / 统计 / 设置"},
					{"Esc", "关闭当前浮层或取消编辑"},
					{"右键任务", "开始专注、加步骤、设闹钟、改标题、删除"},
				} {
					ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
						ui.Text(c, sc[0]).Font(FontMono).FontSize(12.5).FontWeight(600).TextColor(p.BlueDeep).Width(150).SingleLine()
						ui.Text(c, sc[1]).FontSize(13).TextColor(p.Ink2).Grow(1)
					})
				}
			})

			ui.Divider(c)

			// 2) Daily tasks config
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "每日任务").FontSize(16).FontWeight(700).TextColor(p.Navy)
				ui.Text(c, "设置每天都会出现的固定任务，保存后第二天自动出现在首页。").FontSize(13).TextColor(p.Ink3)

				cfg := a.Todo.DailyConfig()
				for _, dt := range cfg.Tasks {
					drow := ui.Row(c.Key("dt-"+dt.ID)).FillWidth().Padding(10, 6).Radius(7).
						BorderWidth(0, 0, 1, 0).BorderColor(p.LineSoft).Transition(colorFadeFast).Gap(8).AlignItems(ui.Center)
					dhov := drow.Hovered() || drow.FocusWithin() || a.editingDid == dt.ID
					if dhov {
						drow.Background(p.AccentSoft)
					}
					drow.Children(func() {
						if a.editingDid == dt.ID {
							in := ui.TextInput(c.Key("dted-"+dt.ID), &a.editingDVal).AutoFocus().Grow(1).Label("编辑每日任务")
							okb := primaryBtn(c, "保存", 12, p).Padding(4, 10)
							canb := ui.Button(c, "取消").Padding(4, 10).FontSize(12)
							if in.Submitted() || okb.Clicked() {
								a.Todo.RenameDaily(dt.ID, a.editingDVal)
								a.editingDid = ""
							} else if canb.Clicked() {
								a.editingDid = ""
							}
						} else {
							ui.Text(c, dt.Title).FontSize(14).FontWeight(500).TextColor(p.Ink).Grow(1)
							if dhov {
								eb := iconBtn(c, IconEdit, "修改每日任务 "+dt.Title, "修改", p).Size(26, 26)
								if eb.Clicked() {
									a.editingDid = dt.ID
									a.editingDVal = dt.Title
								}
								db := iconBtn(c, IconDelete, "删除每日任务 "+dt.Title, "删除", p).Size(26, 26)
								if db.Clicked() {
									a.Todo.RemoveDaily(dt.ID)
								}
							}
						}
					})
				}

				ui.Row(c).FillWidth().Margin(6, 0, 0, 0).Gap(8).Children(func() {
					di := ui.TextInput(c.Key("daily-new-in"), &a.newDailyDraft).Height(40).FontSize(14).Placeholder("例如：睡前读十分钟书").Grow(1).Label("新每日任务")
					db := primaryBtn(c, "添加", 13, p).Height(40).Padding(0, 16)
					if di.Submitted() || db.Clicked() {
						if a.Todo.AddDaily(a.newDailyDraft) != nil {
							a.newDailyDraft = ""
						}
					}
				})
			})

			ui.Divider(c)

			// 3) Markdown Note Templates
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "笔记模板（Obsidian / Notion）").FontSize(16).FontWeight(700).TextColor(p.Navy)
				ui.Text(c, "用 {{变量}} 生成 Markdown 笔记：复制后直接粘贴，或存为 .md 文件。").FontSize(13).TextColor(p.Ink3)
				ui.Segmented(c, &a.tplTab, "每日复盘", "单任务", "周报").Label("选择模板")
				kinds := []string{"daily", "task", "weekly"}
				curKind := kinds[a.tplTab]

				ui.TextArea(c.Key("tpl-ed-"+curKind), &a.tplDraft[a.tplTab]).Font(FontMono).FontSize(12).Height(150).Label("模板内容")
				ui.Row(c).Gap(8).Children(func() {
					saveTpl := primaryBtn(c, "保存模板", 12, p).Padding(5, 12)
					if saveTpl.Clicked() {
						var cur store.ExportTemplate
						_ = a.Store.GetJSON(store.KeyExportTemplate, &cur)
						if cur.Templates == nil {
							cur.Templates = map[string]store.NoteTemplateEntry{}
						}
						defName := notes.DefaultTemplates[curKind].Name
						cur.Templates[curKind] = store.NoteTemplateEntry{Name: defName, Text: a.tplDraft[a.tplTab]}
						a.Store.SetJSON(store.KeyExportTemplate, cur)
						c.Toast("模板已保存")
					}
					prevBtn := ui.Button(c, "预览").Padding(5, 12).FontSize(12)
					if prevBtn.Clicked() {
						req := notes.BuildRequest{Kind: curKind, Day: a.Todo.TodayKey(), TemplateText: a.tplDraft[a.tplTab]}
						if curKind == "task" {
							req.TaskID = "preview-demo"
							req.TaskDay = a.Todo.TodayKey()
						}
						res := notes.BuildNote(a.Store, req, a.now())
						if res.OK {
							a.tplPreview = res.Text
						} else {
							a.tplPreview = notes.Render(a.tplDraft[a.tplTab], notes.RenderData{
								Vars: map[string]string{
									"task_title":    "示例任务",
									"date_cn":       "今天",
									"task_minutes":  "50",
									"focus_minutes": "50",
								},
								Lists: map[string][]map[string]string{
									"timeline": {{"start": "09:00", "end": "09:50", "minutes": "50"}},
								},
							})
						}
						a.tplShowPrev = !a.tplShowPrev
					}
					rstBtn := ui.Button(c, "恢复默认").Padding(5, 12).FontSize(12)
					if rstBtn.Clicked() {
						def := notes.DefaultTemplates[curKind]
						a.tplDraft[a.tplTab] = def.Text
						var cur store.ExportTemplate
						if a.Store.GetJSON(store.KeyExportTemplate, &cur) && cur.Templates != nil {
							delete(cur.Templates, curKind)
							a.Store.SetJSON(store.KeyExportTemplate, cur)
						}
						a.tplShowPrev = false
						c.Toast("已恢复默认模板")
					}
				})
				if a.tplShowPrev && a.tplPreview != "" {
					ui.Column(c).FillWidth().Padding(10).Radius(8).Background(p.Surface).Border(1, p.Line).Children(func() {
						ui.Text(c, a.tplPreview).FontSize(12).TextColor(p.Ink2)
					})
				}
			})

			ui.Divider(c)

			// 4) Data Backup & Diagnostics
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, "数据备份").FontSize(14).FontWeight(600).TextColor(p.Navy)
				ui.Text(c, "数据完全保存在本地磁盘，零网络上传。导入前会自动生成一份备份。").FontSize(12).TextColor(p.Ink3)
				ui.Row(c).Gap(8).Wrap().Children(func() {
					expB := primaryBtn(c, "导出 JSON 备份", 12.5, p).Padding(6, 12)
					if expB.Clicked() {
						a.LaunchExportDialog()
					}
					impB := ui.Button(c, "从备份导入").Padding(6, 12).FontSize(12.5)
					if impB.Clicked() {
						a.LaunchImportPicker(c)
					}
					bakDirB := ui.Button(c, "打开备份文件夹").Padding(6, 12).FontSize(12.5)
					if bakDirB.Clicked() {
						_ = mygo.Shell.OpenPath(a.Store.BackupDir())
					}
					logDirB := ui.Button(c, "打开日志").Padding(6, 12).FontSize(12.5)
					if logDirB.Clicked() {
						mygo.Shell.ShowItemInFolder(a.Log.Path())
					}
					clrB := ui.Button(c, "清空全部数据").Padding(6, 12).FontSize(12.5).TextColor(p.Danger)
					if clrB.Clicked() {
						a.clearAskOpen = true
					}
				})

				rec := a.recoveryInfo()
				ui.Column(c).FillWidth().Padding(12, 14).Radius(10).Background(p.Surface).Border(1, p.LineSoft).Gap(4).Children(func() {
					ui.Textf(c, "版本：526Hz v%s", AppVersion).Font(FontMono).FontSize(11.5).TextColor(p.Ink2)
					ui.Textf(c, "数据文件：%s", a.Store.File()).Font(FontMono).FontSize(11).TextColor(p.Ink3)
					ui.Textf(c, "备份目录：%s（滚动备份 %d 份 / 损坏保留 %d 份）", a.Store.BackupDir(), len(rec.Backups), len(rec.Corrupt)).Font(FontMono).FontSize(11).TextColor(p.Ink3)
					ui.Textf(c, "日志文件：%s", a.Log.Path()).Font(FontMono).FontSize(11).TextColor(p.Ink3)
				})
			})
		})
	})
	})
}

func (a *App) renderImportModal(c *ui.Context, p Palette) {
	if !a.importOpen || a.importSummary == nil {
		return
	}
	s := a.importSummary
	ui.Modal(c, &a.importOpen, func() {
		ui.Column(c).Width(380).Gap(10).Children(func() {
			ui.Text(c, "选择导入方式").FontSize(16).FontWeight(700).TextColor(p.Navy)
			ui.Text(c, "导入前会自动备份当前数据（可随时从备份目录恢复）。请选择导入方式：").FontSize(12.5).TextColor(p.Ink2)
			ui.Column(c).Padding(10, 12).Radius(9).Background(p.Surface).Border(1, p.Line).Gap(3).Children(func() {
				ui.Textf(c, "包含：%d 天 / %d 条任务 / 每日任务 %d 条", s.Days, s.Tasks, s.DailyTemplates).FontSize(12).TextColor(p.Ink)
				ui.Textf(c, "专注历史：%d 条 · 统计：%d 轮 / %d 分钟", s.HistoryEntries, s.TimerRounds, s.TimerMinutes).FontSize(12).TextColor(p.Ink2)
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					a.importOpen = false
					a.importBundle = nil
				}
				rep := ui.Button(c, "整体替换").TextColor(p.Danger)
				if rep.Clicked() {
					a.applyImportMode(c, "replace")
				}
				mrg := primaryBtn(c, "合并导入", 13, p)
				if mrg.Clicked() {
					a.applyImportMode(c, "merge")
				}
			})
		})
	})
}

func (a *App) applyImportMode(c *ui.Context, mode string) {
	raw := a.importBundle
	a.importOpen = false
	a.importBundle = nil
	res := a.Store.ImportAll(raw, mode)
	if !res.OK {
		c.Toast("导入失败：" + res.Error)
		return
	}
	a.Todo.Init()
	a.loadTemplateDrafts()
	a.loadSeaScene()
	a.Timer.ReloadFromStore()
	a.markTotalsDirty()
	a.mu.Lock()
	a.recoveryDirty = true
	a.mu.Unlock()
	c.Toast(fmt.Sprintf("已导入 %d 项数据", res.Applied))
}

func (a *App) renderClearAlert(c *ui.Context) {
	if !a.clearAskOpen {
		return
	}
	choice := ui.AlertDialog(c, &a.clearAskOpen,
		"清空全部数据？",
		"将清空今日事任务、每日任务、专注统计、专注历史与偏好设置。清空前会自动生成 pre-clear 备份。",
		"取消", "确认清空",
	)
	if choice == 1 {
		res := a.Store.Clear(nil)
		if !res.OK {
			c.Toast("清空失败：" + res.Error)
			return
		}
		a.Todo.Init()
		a.loadTemplateDrafts()
		a.Timer.ReloadFromStore()
		a.markTotalsDirty()
		a.mu.Lock()
		a.recoveryDirty = true
		a.mu.Unlock()
		preBak := res.PreBackup
		c.ToastAction(fmt.Sprintf("已清空 %d 项数据", res.Removed), "打开备份", func() {
			if preBak != "" {
				mygo.Shell.ShowItemInFolder(preBak)
			}
		})
	}
}
