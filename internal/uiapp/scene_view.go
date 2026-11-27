package uiapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Stars76/526Hz/internal/store"
	"github.com/Stars76/526Hz/internal/weather"
	"github.com/egoist/mygo/ui"
)

var builtInFlowPresets = []store.FlowPreset{
	{Name: "晴海", Theme: "light", WeatherMode: "clear", FocusMin: 25, AmbientOn: true, AmbientVolume: 0.42},
	{Name: "雨夜", Theme: "dark", WeatherMode: "rain", FocusMin: 45, AmbientOn: true, AmbientVolume: 0.48},
	{Name: "雷暴", Theme: "dark", WeatherMode: "storm", FocusMin: 45, AmbientOn: true, AmbientVolume: 0.52},
}

func ambientSoundLabel(w string) string {
	switch w {
	case "rain":
		return "细雨"
	case "storm":
		return "雷雨"
	case "snow", "fog":
		return "微风"
	default:
		return "海浪"
	}
}

// renderSeaSceneModal renders the minimalist "海域场景" modal:
// presets, weather (+ optional Open-Meteo live sync), ambient white noise, lighting/egg toggles, and 6-item Atlas.
func (a *App) renderSeaSceneModal(c *ui.Context, p Palette) {
	if !a.sceneModalOpen {
		return
	}
	sc := a.seaSceneSnapshot()
	_, periodLabel := DayPeriod(a.now())
	effWeather := a.effectiveWeather()

	a.mu.Lock()
	syncBusy := a.sceneSyncBusy
	syncErr := a.sceneSyncErr
	a.mu.Unlock()

	ui.Box(c.Key("sea-scene-backdrop")).Absolute().Top(0).Right(0).Bottom(0).Left(0).ZIndex(12).
		Background(p.ModalBackdrop).Center().Padding(16).Children(func() {
		card := ui.Column(c.Key("sea-scene-card")).Width(540).MaxWidthPercent(94).MaxHeight(660).
			Padding(16, 18).Radius(16).Background(p.Paper).Border(1, p.Line).
			Shadow(0, 18, 44, 0, p.ElevatedShadow).Transition(timerOverlayMotion).Gap(10)
		if card.PressedOutside() {
			a.sceneModalOpen = false
		}
		card.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, IconScene).Size(17, 17).TextColor(p.Blue)
				ui.Text(c, "海域场景").FontSize(16).FontWeight(800).TextColor(p.Navy).Grow(1)
				cls := timerBarBtn(c, IconClose, "关闭海域场景", "关闭", false, false, p).Size(26, 26)
				if cls.Clicked() {
					a.sceneModalOpen = false
				}
			})

			ui.Scroll(c.Key("sea-scene-scroll")).Grow(1).MinHeight(0).Gap(12).Children(func() {
				// 1) Presets
				ui.Column(c).FillWidth().Gap(7).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, "场景预设").FontSize(13).FontWeight(700).TextColor(p.Navy).Grow(1)
						savePre := ui.ButtonBase(c).Height(24).Padding(0, 9).Radius(999).
							Background(p.Surface).Border(1, p.Line).Transition(colorFadeFast).
							Cursor(ui.CursorPointer).Label("保存当前为我的预设")
						if savePre.Hovered() {
							savePre.Border(1, p.Blue)
						}
						savePre.Children(func() {
							ui.Text(c, "保存当前").FontSize(11).FontWeight(600).TextColor(p.BlueDeep).SingleLine()
						})
						if savePre.Clicked() {
							a.saveCustomFlowPreset()
							c.Toast("已保存")
						}
					})

					presets := make([]store.FlowPreset, 0, 4)
					presets = append(presets, builtInFlowPresets...)
					if sc.CustomPreset != nil {
						presets = append(presets, *sc.CustomPreset)
					}
					ui.Grid(c).Columns(len(presets)).Gap(6).FillWidth().Children(func() {
						for idx, pr := range presets {
							prCopy := pr
							sub := fmt.Sprintf("%s · %dm", weather.WeatherLabel(prCopy.WeatherMode), prCopy.FocusMin)
							b := ui.ButtonBase(c.Key(fmt.Sprintf("preset-%d-%s", idx, prCopy.Name))).Padding(7, 8).Radius(10).
								Background(p.Surface).Border(1, p.Line).Transition(colorFadeFast).
								Cursor(ui.CursorPointer).Label("预设 " + prCopy.Name)
							if b.Hovered() {
								b.Background(p.AccentSoft).Border(1, p.Blue)
							}
							b.Children(func() {
								ui.Column(c).Center().Gap(2).Children(func() {
									ui.Text(c, prCopy.Name).FontSize(12.5).FontWeight(700).TextColor(p.Navy).SingleLine()
									ui.Text(c, sub).FontSize(10.5).TextColor(p.Ink3).SingleLine()
								})
							})
							if b.Clicked() {
								a.applyFlowPreset(prCopy)
							}
						}
					})
				})

				ui.Divider(c)

				// 2) Weather
				ui.Column(c).FillWidth().Gap(7).Children(func() {
					ui.Text(c, "天气").FontSize(13).FontWeight(700).TextColor(p.Navy)
					weatherOpts := []struct {
						key   string
						label string
					}{
						{"clear", "晴空"},
						{"rain", "细雨"},
						{"snow", "飞雪"},
						{"storm", "雷雨"},
						{"fog", "薄雾"},
						{"live", "实况"},
					}
					ui.Grid(c).Columns(6).Gap(5).FillWidth().Children(func() {
						for _, opt := range weatherOpts {
							active := sc.WeatherMode == opt.key
							b := ui.ButtonBase(c.Key("wopt-"+opt.key)).Height(30).Radius(8).Center().
								Background(p.Surface).Border(1, p.Line).Transition(colorFadeFast).
								Cursor(ui.CursorPointer).Label("天气 " + opt.label)
							if active {
								b.Background(p.AccentSoft).Border(1.5, p.Blue)
							} else if b.Hovered() {
								b.Border(1, p.Blue)
							}
							b.Children(func() {
								col := p.Ink2
								w := 600
								if active {
									col = p.BlueDeep
									w = 700
								}
								ui.Text(c, opt.label).FontSize(12).FontWeight(w).TextColor(col).SingleLine()
							})
							if b.Clicked() {
								k := opt.key
								a.updateSeaScene(func(cfg *store.SeaSceneConfig) {
									cfg.WeatherMode = k
								})
								if k == "live" && strings.TrimSpace(a.sceneCityDraft) != "" {
									a.syncLiveWeatherAsync(a.sceneCityDraft, false)
								}
							}
						}
					})

					if sc.WeatherMode == "live" {
						ui.Column(c).FillWidth().Padding(9, 11).Radius(10).Background(p.Surface).Border(1, p.LineSoft).Gap(6).Children(func() {
							ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
								in := ui.TextInput(c.Key("scene-city-in"), &a.sceneCityDraft).
									Height(30).FontSize(12.5).Placeholder("输入城市名（如：杭州、北京）").Grow(1).Label("天气城市")
								btnLbl := "同步"
								if syncBusy {
									btnLbl = "同步中…"
								}
								syncB := primaryBtn(c, btnLbl, 12, p).Height(30).Padding(0, 12).Disabled(syncBusy)
								if in.Submitted() || syncB.Clicked() {
									a.syncLiveWeatherAsync(a.sceneCityDraft, true)
								}
							})
							if syncErr != "" {
								ui.Text(c, syncErr).FontSize(11).TextColor(p.Danger)
							} else if sc.ResolvedCity != "" {
								tempStr := ""
								if sc.LiveTempC != nil {
									tempStr = " " + formatTemp(*sc.LiveTempC)
								}
								ui.Textf(c, "%s · %s%s", sc.ResolvedCity, weather.WeatherLabel(sc.LiveWeather), tempStr).
									FontSize(11.5).FontWeight(600).TextColor(p.BlueDeep)
							}
						})
					}
				})

				ui.Divider(c)

				// 3) Ambient white noise (follows focus timer state)
				ui.Column(c).FillWidth().Gap(7).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "环境音").FontSize(13).FontWeight(700).TextColor(p.Navy)
								badgeChip(c, ambientSoundLabel(effWeather), p.AccentSoft, p.BlueDeep, ui.Transparent)
							})
							ui.Text(c, "专注计时中自动播放").FontSize(11).TextColor(p.Ink3)
						})
						ao := sc.AmbientOn
						if ui.Switch(c.Key("sc-ambient-on"), &ao).Label("环境音").Changed() {
							a.updateSeaScene(func(cfg *store.SeaSceneConfig) {
								cfg.AmbientOn = ao
							})
							// If toggled ON while idle, immediately play a 5s confirmation preview so the user hears it right away!
							if ao && a.Ambient != nil && !a.Timer.Snapshot().Timer.Running {
								a.Ambient.Preview(effWeather, sc.AmbientVolume, 5*time.Second)
							} else if !ao && a.Ambient != nil {
								a.Ambient.StopPreview()
							}
						}
					})

					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "音量").FontSize(11.5).TextColor(p.Ink3)
						vol := sc.AmbientVolume
						if ui.Slider(c.Key("sc-ambient-vol"), &vol, 0.1, 1.0).Grow(1).Label("环境音量").Changed() {
							a.updateSeaScene(func(cfg *store.SeaSceneConfig) {
								cfg.AmbientVolume = vol
							})
						}
						ui.Textf(c, "%d%%", int(vol*100+0.5)).Font(FontMono).FontSize(11).TextColor(p.Ink3).Width(34)
						isPrev := a.Ambient != nil && a.Ambient.IsPreviewing()
						prevLbl := "试听"
						if isPrev {
							prevLbl = "停止试听"
						}
						pvBtn := ui.Button(c, prevLbl).Height(26).Padding(0, 10).Radius(7).FontSize(11.5)
						if pvBtn.Clicked() && a.Ambient != nil {
							if isPrev {
								a.Ambient.StopPreview()
							} else {
								a.Ambient.Preview(effWeather, sc.AmbientVolume, 5*time.Second)
							}
						}
					})
				})

				ui.Divider(c)

				// 4) Time-of-day & Easter Eggs toggles
				ui.Row(c).FillWidth().Gap(10).Children(func() {
					ui.Row(c).Grow(1).Padding(8, 10).Radius(9).Background(p.Surface).Border(1, p.LineSoft).
						AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, "天色光线").FontSize(12.5).FontWeight(700).TextColor(p.Navy)
							ui.Text(c, periodLabel).FontSize(11).TextColor(p.BlueDeep)
						})
						tl := sc.TimeLighting
						if ui.Switch(c.Key("sc-time-light"), &tl).Label("天色光线").Changed() {
							a.updateSeaScene(func(cfg *store.SeaSceneConfig) {
								cfg.TimeLighting = tl
							})
						}
					})

					ui.Row(c).Grow(1).Padding(8, 10).Radius(9).Background(p.Surface).Border(1, p.LineSoft).
						AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Column(c).Grow(1).Gap(2).Children(func() {
							ui.Text(c, "海面彩蛋").FontSize(12.5).FontWeight(700).TextColor(p.Navy)
							ui.Text(c, "随机出现").FontSize(11).TextColor(p.Ink3)
						})
						eo := sc.EncountersOn
						if ui.Switch(c.Key("sc-encounters"), &eo).Label("海面彩蛋").Changed() {
							a.updateSeaScene(func(cfg *store.SeaSceneConfig) {
								cfg.EncountersOn = eo
							})
							if !eo {
								a.stageSea.ClearEncounter()
							}
						}
					})
				})

				ui.Divider(c)

				// 5) 6-Card Minimalist Atlas
				ui.Column(c).FillWidth().Gap(8).Children(func() {
					unlocked := 0
					for _, m := range AllEncounters {
						if sc.Sightings[m.Kind] > 0 {
							unlocked++
						}
					}
					ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, "图鉴").FontSize(13).FontWeight(700).TextColor(p.Navy).Grow(1)
						badgeChip(c, fmt.Sprintf("%d / %d", unlocked, len(AllEncounters)), p.AccentSoft, p.BlueDeep, ui.Transparent)
					})

					ui.Grid(c).Columns(2).Gap(7).FillWidth().Children(func() {
						for _, meta := range AllEncounters {
							mCopy := meta
							cnt := sc.Sightings[mCopy.Kind]
							isActive := a.stageSea.ActiveEncounterKind(a.now()) == mCopy.Kind

							cardBg := p.Surface
							cardBd := p.LineSoft
							if isActive {
								cardBg = p.AccentSoft
								cardBd = p.Blue
							}
							ui.Column(c.Key("atlas-"+mCopy.Kind)).FillWidth().Padding(8, 10).Radius(10).
								Background(cardBg).Border(1, cardBd).Gap(5).Children(func() {
								ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
									ui.Text(c, mCopy.Name).FontSize(12.5).FontWeight(700).TextColor(p.Navy).Grow(1).SingleLine()
									if isActive {
										badgeChip(c, "出现中", p.SuccessSoft, p.Success, p.SuccessLine)
									} else if cnt > 0 {
										badgeChip(c, fmt.Sprintf("%d次", cnt), p.Paper, p.BlueDeep, p.Line)
									}
								})
								ui.Text(c, mCopy.Preview).Font(FontMono).FontSize(10).TextColor(p.BlueDeep).SingleLine()
								ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
									ui.Text(c, mCopy.Hint).FontSize(10.5).TextColor(p.Ink4).Grow(1).SingleLine()
									sumBtn := ui.ButtonBase(c.Key("summon-"+mCopy.Kind)).Height(24).Padding(0, 9).Radius(6).
										Background(p.Paper).Border(1, p.Line).Transition(colorFadeFast).
										Cursor(ui.CursorPointer).Label("召唤 " + mCopy.Name)
									if sumBtn.Hovered() {
										sumBtn.Border(1, p.Blue)
									}
									sumBtn.Children(func() {
										ui.Text(c, "召唤").FontSize(11).FontWeight(600).TextColor(p.BlueDeep).SingleLine()
									})
									if sumBtn.Clicked() {
										a.summonEncounter(mCopy.Kind)
										a.sceneModalOpen = false
									}
								})
							})
						}
					})
				})
			})

			ui.Row(c).FillWidth().Padding(2, 0, 0, 0).Gap(8).AlignItems(ui.Center).Children(func() {
				rndBtn := ui.Button(c, "随机召唤").Height(32).Padding(0, 14).Radius(9).FontSize(12)
				if rndBtn.Clicked() {
					a.summonEncounter("")
					a.sceneModalOpen = false
				}
				ui.Spacer(c)
				doneBtn := primaryBtn(c, "完成", 12, p).Height(32).Padding(0, 18).Radius(9)
				if doneBtn.Clicked() {
					a.sceneModalOpen = false
				}
			})
		})
	})
}
