package uiapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Stars76/526Hz/internal/assets"
	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/notes"
	"github.com/Stars76/526Hz/internal/sound"
	"github.com/Stars76/526Hz/internal/stats"
	"github.com/Stars76/526Hz/internal/store"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/Stars76/526Hz/internal/todo"
	"github.com/Stars76/526Hz/internal/weather"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const AppVersion = "3.0.0"

// App coordinates domain services, native windows, tray, audio, and UI state.
type App struct {
	Store   *store.Store
	Log     *store.Logger
	Timer   *timer.Engine
	Alarm   *timer.AlarmScheduler
	Todo    *todo.Service
	Sound   *sound.Player
	Ambient *sound.AmbientEngine
	Weather *weather.Client

	MainWin  *mygo.Window
	TimerWin *mygo.Window
	Tray     *TrayController

	stageSea   *SeaState
	miniSea    *SeaState
	keepRunRel func()
	quitting   bool
	trayHinted bool
	nowFn      func() time.Time
	customNow  bool

	// Main window navigation & modals
	activeOverlay    string // "" | "focus" | "calendar" | "settings" | "stats"
	sidebarCollapsed bool
	onlyOpen         bool
	doneGroupOpen    bool
	collapsed        map[string]bool
	calMonth         time.Time
	tlDrawerOpen     bool
	checkRippleID    string
	checkRippleAt    time.Time
	autostartLoaded  bool
	autostartOn      bool

	// Todo interactions
	newTaskDraft   string
	editingTaskID  string
	editingTaskVal string
	subAddTaskID   string
	subAddDraft    string
	editingSubKey  string // taskID/subID
	editingSubVal  string
	alarmTaskID    string
	alarmTimeVal   time.Time
	alarmInputStr  string
	todoScroll     ui.ScrollState
	lastUndoTask   *store.Task
	lastUndoIndex  int
	bindPickOpen   bool      // Today page: "which task is this round for?" picker
	resetArmedAt   time.Time // first click on 重置 during an active round arms a confirm step

	// Settings view
	newDailyDraft string
	editingDid    string
	editingDVal   string
	tplTab        int // 0=daily, 1=task, 2=weekly
	tplDraft      [3]string
	tplPreview    string
	tplShowPrev   bool
	settingsScrl  ui.ScrollState

	// Data import / clear dialogs
	importOpen    bool
	importSummary *store.ImportSummary
	importBundle  []byte
	clearAskOpen  bool

	// Statistics & daily timeline
	statsTab         int // 0=day, 1=week, 2=month
	tlDayKey         string
	tlScroll         ui.ScrollState
	tlScrolledDay    string
	tlFormOpen       bool
	tlFormMode       string // "add" | "edit"
	tlEditKey        string
	tlFormTitle      string
	tlFormStart      string
	tlFormEnd        string
	tlActionKey      string // currently clicked block key for quick edit/hide/note
	statsScroll      ui.ScrollState
	cachedOverrides  store.TimelineOverrides
	overridesDirty   bool
	cachedTlDay      string
	cachedTlBlocks   []stats.TimelineBlock
	cachedTlHidden   []string
	tlCacheDirty     bool
	cachedSumPeriod  string
	cachedSumDay     string
	cachedSummary    stats.PeriodSummary
	summaryDirty     bool
	cachedYearDay    string
	cachedYearSum    stats.YearSummary
	yearDirty        bool
	cachedHistory    []store.HistoryEntry
	historyDirty     bool
	statsOnlyUnbound bool
	statsShowAllRecs bool
	lastUndoHistory  []store.HistoryEntry

	// Compact timer window overlays & completion screen
	timerMiniPill    bool
	timerShowSetup   bool
	timerShowSet     bool
	timerCustomMin   string
	timerDoneVisible bool
	timerDoneEv      *timer.Event
	timerDoneDetail  bool
	confettiAt       time.Time

	// ASCII Sea Scene (weather, time-of-day lighting, random encounters & atlas)
	seaScene       store.SeaSceneConfig
	sceneModalOpen bool
	sceneCityDraft string
	sceneSyncBusy  bool
	sceneSyncErr   string
	weatherStopCh  chan struct{}

	// Pending cross-thread notifications/toasts and frame caches
	mu              sync.Mutex
	pendingToast    string
	pendingUndo     func()
	sysDark         bool
	hasSysDark      bool
	cachedTotals    map[string]int
	totalsDirty     bool
	cachedRecovery  store.RecoveryInfo
	recoveryDirty   bool
	lastMainChipSig string
}

type AppConfig struct {
	UserDataDir string
	Now         func() time.Time
}

func NewApp(cfg AppConfig) *App {
	dir := cfg.UserDataDir
	logDir := filepath.Join(dir, "logs")
	logger := store.NewLogger(logDir)
	st := store.NewStore(dir, logger)
	loadRes := st.Load()
	logger.Info("存储文件:", st.File(), loadRes.Loaded)
	st.RollingBackup()

	customNow := cfg.Now != nil
	nowFn := cfg.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	a := &App{
		Store:          st,
		Log:            logger,
		Sound:          sound.NewPlayer(),
		Ambient:        sound.NewAmbientEngine(),
		Weather:        weather.NewClient(),
		stageSea:       NewSeaState(),
		miniSea:        NewSeaState(),
		nowFn:          nowFn,
		customNow:      customNow,
		collapsed:      map[string]bool{},
		calMonth:       time.Date(nowFn().Year(), nowFn().Month(), 1, 12, 0, 0, 0, time.Local),
		tlDayKey:       dateutil.TodayKey(nowFn()),
		totalsDirty:    true,
		recoveryDirty:  true,
		overridesDirty: true,
		tlCacheDirty:   true,
		summaryDirty:   true,
		yearDirty:      true,
		historyDirty:   true,
	}

	a.Todo = todo.NewService(st, logger, nowFn)
	a.Todo.Init()
	vp := a.Todo.ViewPrefs()
	a.onlyOpen = vp.OnlyOpen
	for _, id := range vp.Collapsed {
		a.collapsed[id] = true
	}
	a.loadTemplateDrafts()
	a.loadSeaScene()

	a.Timer = timer.NewEngine(timer.Options{
		Store: st,
		Log:   logger,
		Now:   nowFn,
		OnBroadcast: func(snap timer.Snapshot) {
			a.onTimerBroadcast(snap)
		},
		OnEvent: func(ev timer.Event) {
			a.onTimerEvent(ev)
		},
		Notify: func(title, body string) {
			a.SendNotification(title, body)
		},
	})
	a.Alarm = timer.NewAlarmScheduler(st, logger, func(title, body string) {
		a.SendNotification(title, body)
	}, nowFn)

	a.Timer.Init()
	return a
}

func (a *App) now() time.Time {
	return a.nowFn()
}

// liveNow returns a deterministic timestamp only in tests with an injected clock;
// in the live desktop app it returns zero so 60fps GPU paint callbacks sample live time.Now() every frame.
func (a *App) liveNow() time.Time {
	if a.customNow {
		return a.nowFn()
	}
	return time.Time{}
}

func (a *App) loadSeaScene() {
	sc := store.DefaultSeaSceneConfig()
	if a.Store.GetJSON(store.KeySeaScene, &sc) {
		sc.Normalize()
	}
	a.mu.Lock()
	a.seaScene = sc
	a.sceneCityDraft = sc.City
	a.mu.Unlock()
}

func (a *App) seaSceneSnapshot() store.SeaSceneConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	cp := a.seaScene
	cp.Sightings = make(map[string]int, len(a.seaScene.Sightings))
	for k, v := range a.seaScene.Sightings {
		cp.Sightings[k] = v
	}
	cp.LastSeen = make(map[string]string, len(a.seaScene.LastSeen))
	for k, v := range a.seaScene.LastSeen {
		cp.LastSeen[k] = v
	}
	return cp
}

func (a *App) updateSeaScene(fn func(*store.SeaSceneConfig)) {
	a.mu.Lock()
	fn(&a.seaScene)
	a.seaScene.Normalize()
	cp := a.seaScene
	a.mu.Unlock()
	a.Store.SetJSON(store.KeySeaScene, cp)
	if a.Timer != nil {
		a.syncAmbientAudio(a.Timer.Snapshot())
	}
	a.InvalidateAll()
}

func (a *App) syncAmbientAudio(snap timer.Snapshot) {
	if a.Ambient == nil {
		return
	}
	sc := a.seaSceneSnapshot()
	wantPlay := sc.AmbientOn && snap.Timer.Running && snap.Timer.Phase == "focus"
	a.Ambient.Sync(wantPlay, a.effectiveWeather(), sc.AmbientVolume)
}

func (a *App) handleOceanClick(px, py float32) {
	kind, snd, caught := a.stageSea.HandleOceanClick(px, py, a.now())
	if kind == "" {
		return
	}
	if a.Timer != nil && a.Timer.Prefs().Sound && snd != "" {
		a.Sound.Play(snd)
	}
	if caught {
		a.updateSeaScene(func(sc *store.SeaSceneConfig) {
			sc.FishCaught++
		})
	} else {
		a.InvalidateAll()
	}
}

func (a *App) applyFlowPreset(p store.FlowPreset) {
	theme := p.Theme
	if theme != "" {
		a.Timer.SetPrefs(timer.PrefsPatch{Theme: &theme})
	}
	if p.FocusMin >= 1 && p.FocusMin <= 180 {
		snap := a.Timer.Snapshot()
		if snap.Timer.Idle {
			a.Timer.Command("setDuration", timer.CmdPayload{Sec: p.FocusMin * 60})
		}
	}
	a.updateSeaScene(func(sc *store.SeaSceneConfig) {
		if p.WeatherMode != "" {
			sc.WeatherMode = p.WeatherMode
		}
		sc.AmbientOn = p.AmbientOn
		if p.AmbientVolume > 0 && p.AmbientVolume <= 1 {
			sc.AmbientVolume = p.AmbientVolume
		}
	})
}

func (a *App) saveCustomFlowPreset() {
	snap := a.Timer.Snapshot()
	mins := (snap.Phases.Focus + 30) / 60
	if mins < 1 {
		mins = 25
	}
	a.updateSeaScene(func(sc *store.SeaSceneConfig) {
		sc.CustomPreset = &store.FlowPreset{
			Name:          "我的预设",
			Theme:         snap.Prefs.Theme,
			WeatherMode:   sc.WeatherMode,
			FocusMin:      mins,
			AmbientOn:     sc.AmbientOn,
			AmbientVolume: sc.AmbientVolume,
		}
	})
}

func (a *App) effectiveWeather() string {
	a.mu.Lock()
	mode := a.seaScene.WeatherMode
	live := a.seaScene.LiveWeather
	a.mu.Unlock()
	if mode == "live" {
		if live != "" {
			return live
		}
		return "clear"
	}
	if mode == "" {
		return "clear"
	}
	return mode
}

func (a *App) recordNaturalEncounter(kind string) {
	if kind == "" {
		return
	}
	today := a.Todo.TodayKey()
	a.updateSeaScene(func(sc *store.SeaSceneConfig) {
		sc.Sightings[kind]++
		sc.LastSeen[kind] = today
	})
}

func (a *App) summonEncounter(kind string) string {
	now := a.now()
	chosen := a.stageSea.TriggerEncounter(kind, now, false)
	a.InvalidateAll()
	return chosen
}

func (a *App) syncLiveWeatherAsync(city string, forceGeocode bool) {
	q := strings.TrimSpace(city)
	if q == "" {
		a.mu.Lock()
		a.sceneSyncErr = "请先输入城市名（如：杭州、北京、Tokyo）"
		a.sceneSyncBusy = false
		a.mu.Unlock()
		a.InvalidateAll()
		return
	}
	a.mu.Lock()
	if a.sceneSyncBusy {
		a.mu.Unlock()
		return
	}
	a.sceneSyncBusy = true
	a.sceneSyncErr = ""
	cachedCity := a.seaScene.City
	cachedResolved := a.seaScene.ResolvedCity
	cachedLat := a.seaScene.Lat
	cachedLon := a.seaScene.Lon
	a.mu.Unlock()
	a.InvalidateAll()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()

		lat, lon, resolved := cachedLat, cachedLon, cachedResolved
		if forceGeocode || q != cachedCity || resolved == "" || (lat == 0 && lon == 0) {
			geo, err := a.Weather.Geocode(ctx, q)
			if err != nil {
				a.mu.Lock()
				a.sceneSyncBusy = false
				a.sceneSyncErr = err.Error()
				a.mu.Unlock()
				a.InvalidateAll()
				return
			}
			lat, lon, resolved = geo.Lat, geo.Lon, geo.Name
		}
		cur, err := a.Weather.FetchCurrent(ctx, lat, lon)
		if err != nil {
			a.mu.Lock()
			a.sceneSyncBusy = false
			a.sceneSyncErr = err.Error()
			a.mu.Unlock()
			a.InvalidateAll()
			return
		}
		tempVal := cur.TempC
		a.mu.Lock()
		a.sceneSyncBusy = false
		a.sceneSyncErr = ""
		a.seaScene.City = q
		a.seaScene.ResolvedCity = resolved
		a.seaScene.Lat = lat
		a.seaScene.Lon = lon
		a.seaScene.LiveWeather = cur.Weather
		a.seaScene.LiveTempC = &tempVal
		a.seaScene.LiveUpdatedAt = cur.FetchedAt.UTC().Format(time.RFC3339)
		a.seaScene.WeatherMode = "live"
		a.seaScene.Normalize()
		cp := a.seaScene
		a.mu.Unlock()

		a.Store.SetJSON(store.KeySeaScene, cp)
		a.SetPendingToast(fmt.Sprintf("已同步 %s 实况：%s %s", resolved, weather.WeatherLabel(cur.Weather), formatTemp(cur.TempC)), nil)
	}()
}

func (a *App) startWeatherLoop() {
	a.mu.Lock()
	if a.weatherStopCh != nil {
		a.mu.Unlock()
		return
	}
	stopCh := make(chan struct{})
	a.weatherStopCh = stopCh
	mode := a.seaScene.WeatherMode
	city := a.seaScene.City
	a.mu.Unlock()

	if mode == "live" && strings.TrimSpace(city) != "" {
		a.syncLiveWeatherAsync(city, false)
	}

	go func() {
		tk := time.NewTicker(45 * time.Minute)
		defer tk.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-tk.C:
				sc := a.seaSceneSnapshot()
				if sc.WeatherMode == "live" && strings.TrimSpace(sc.City) != "" {
					a.syncLiveWeatherAsync(sc.City, false)
				}
			}
		}
	}()
}

func (a *App) stopWeatherLoop() {
	a.mu.Lock()
	ch := a.weatherStopCh
	a.weatherStopCh = nil
	a.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (a *App) loadTemplateDrafts() {
	var stored store.ExportTemplate
	has := a.Store.GetJSON(store.KeyExportTemplate, &stored)
	var ptr *store.ExportTemplate
	if has {
		ptr = &stored
	}
	kinds := []string{"daily", "task", "weekly"}
	for i, k := range kinds {
		a.tplDraft[i] = notes.Pick(ptr, k).Text
	}
}

func (a *App) saveTodoViewPrefs() {
	col := make([]string, 0, len(a.collapsed))
	for k, v := range a.collapsed {
		if v {
			col = append(col, k)
		}
	}
	a.Todo.SaveViewPrefs(store.TodoViewPrefs{OnlyOpen: a.onlyOpen, Collapsed: col})
}

func (a *App) SendNotification(title, body string) {
	if !mygo.App.IsReady() || !mygo.NotificationsSupported() {
		return
	}
	go func() {
		n := mygo.NewNotification(mygo.NotificationOptions{
			Title:  title,
			Body:   body,
			Silent: true,
		})
		_ = n.Show()
	}()
}

func (a *App) updateKeepRunning(running bool) {
	if !mygo.App.IsReady() {
		return
	}
	if running && a.keepRunRel == nil {
		a.keepRunRel = mygo.Power.KeepRunning("526Hz 专注计时中")
	} else if !running && a.keepRunRel != nil {
		rel := a.keepRunRel
		a.keepRunRel = nil
		rel()
	}
}

func (a *App) markTotalsDirty() {
	a.mu.Lock()
	a.totalsDirty = true
	a.tlCacheDirty = true
	a.summaryDirty = true
	a.yearDirty = true
	a.historyDirty = true
	a.mu.Unlock()
}

func (a *App) markStatsDirty() {
	a.mu.Lock()
	a.overridesDirty = true
	a.tlCacheDirty = true
	a.summaryDirty = true
	a.yearDirty = true
	a.historyDirty = true
	a.mu.Unlock()
}

func (a *App) historyCached() []store.HistoryEntry {
	a.mu.Lock()
	if !a.historyDirty {
		h := a.cachedHistory
		a.mu.Unlock()
		return h
	}
	a.mu.Unlock()

	h := a.Timer.History()
	a.mu.Lock()
	a.cachedHistory = h
	a.historyDirty = false
	a.mu.Unlock()
	return h
}

func (a *App) recoveryInfo() store.RecoveryInfo {
	a.mu.Lock()
	if !a.recoveryDirty {
		rec := a.cachedRecovery
		a.mu.Unlock()
		return rec
	}
	a.mu.Unlock()

	rec := a.Store.ListRecovery()
	a.mu.Lock()
	a.cachedRecovery = rec
	a.recoveryDirty = false
	a.mu.Unlock()
	return rec
}

func (a *App) taskTotals() map[string]int {
	a.mu.Lock()
	if !a.totalsDirty && a.cachedTotals != nil {
		res := a.cachedTotals
		a.mu.Unlock()
		return res
	}
	a.mu.Unlock()

	var hist []store.HistoryEntry
	_ = a.Store.GetJSON(store.KeyTimerHistory, &hist)
	totals := stats.TaskTotals(hist)

	a.mu.Lock()
	a.cachedTotals = totals
	a.totalsDirty = false
	a.mu.Unlock()
	return totals
}

func mainChipSignature(snap timer.Snapshot) string {
	t := snap.Timer
	taskKey := ""
	if t.Task != nil {
		taskKey = t.Task.Day + "|" + t.Task.ID + "|" + t.Task.Title
	}
	return fmt.Sprintf("%s|%d|%v|%v|%v|%v|%s|%s|%v",
		t.Phase, t.TotalSec, t.Running, t.Paused, t.Idle, t.Recovered, taskKey, snap.Prefs.Theme, snap.Prefs.CloseToTray)
}

// todayTideLevel returns a normalized [0.26..0.96] tide level combining today's task progress and focus minutes.
func (a *App) todayTideLevel(snap timer.Snapshot) (level float64, doneCount, totalCount int) {
	tasks := a.Todo.TasksForDay(a.Todo.TodayKey())
	totalCount = len(tasks)
	for _, t := range tasks {
		if t.Done {
			doneCount++
		}
	}
	taskRatio := 0.0
	if totalCount > 0 {
		taskRatio = float64(doneCount) / float64(totalCount)
	}
	focusRatio := float64(snap.Stats.Minutes) / 120.0
	if focusRatio > 1 {
		focusRatio = 1
	}
	combined := taskRatio*0.58 + focusRatio*0.42
	level = 0.26 + combined*0.70
	return level, doneCount, totalCount
}

func (a *App) onTimerBroadcast(snap timer.Snapshot) {
	a.updateKeepRunning(snap.Timer.Running)
	a.syncAmbientAudio(snap)
	if a.Tray != nil {
		a.Tray.Refresh(snap)
	}
	SyncNativeThemeMode(snap.Prefs.Theme)

	sig := mainChipSignature(snap)
	a.mu.Lock()
	mainChanged := sig != a.lastMainChipSig
	if mainChanged {
		a.lastMainChipSig = sig
	}
	a.mu.Unlock()

	liveMain := a.activeOverlay == "focus" || (a.activeOverlay == "" && snap.Timer.Running)
	if (mainChanged || liveMain) && a.MainWin != nil && !a.MainWin.IsDestroyed() && a.MainWin.IsVisible() {
		a.MainWin.Invalidate()
	}
	if a.TimerWin != nil && !a.TimerWin.IsDestroyed() && a.TimerWin.IsVisible() {
		a.TimerWin.Invalidate()
	}
}

func (a *App) onTimerEvent(ev timer.Event) {
	switch ev.Type {
	case "await-plan":
		mygo.RunOnMain(func() {
			a.timerShowSetup = true
			a.timerDoneVisible = false
			if a.MainWin != nil && !a.MainWin.IsDestroyed() && a.MainWin.IsVisible() {
				a.activeOverlay = "focus"
				a.MainWin.Focus()
			} else {
				a.OpenTimerWindow()
			}
			a.InvalidateAll()
		})
	case "plan-advance":
		if a.Timer.Prefs().Sound {
			chime := "break"
			if ev.PrevPhase == "focus" || (ev.PrevPhase == "" && ev.Phase != "focus") {
				chime = "focus"
			}
			a.Sound.Play(chime)
		}
		phaseWord := "休息"
		if ev.Phase == "focus" {
			phaseWord = "专注"
		}
		a.SetPendingToast(fmt.Sprintf("%s · 第 %d/%d 段：%s %d 分钟", ev.Name, ev.Index+1, ev.Total, phaseWord, ev.Minutes), nil)
		a.InvalidateAll()
	case "done":
		a.markTotalsDirty()
		if !ev.Silent {
			if a.Timer.Prefs().Sound {
				if ev.Phase == "focus" {
					a.Sound.Play("focus")
				} else {
					a.Sound.Play("break")
				}
			}
			// Reward completed focus rounds with a natural celebratory DeepSeek Whale (or ocean easter egg)!
			if ev.Phase == "focus" && a.seaSceneSnapshot().EncountersOn {
				kind := a.stageSea.TriggerEncounter(EncounterWhale, a.now(), true)
				a.recordNaturalEncounter(kind)
			}
			evCopy := ev
			mygo.RunOnMain(func() {
				a.timerDoneEv = &evCopy
				a.timerDoneVisible = true
				a.timerDoneDetail = false
				a.confettiAt = time.Now()
				if a.timerMiniPill {
					a.SetTimerMiniPill(false)
				}
				a.InvalidateAll()
			})
		} else {
			a.InvalidateAll()
		}
	case "abandoned", "day-rollover":
		a.markTotalsDirty()
		a.InvalidateAll()
	}
}

func (a *App) SetPendingToast(msg string, undo func()) {
	a.mu.Lock()
	a.pendingToast = msg
	a.pendingUndo = undo
	a.mu.Unlock()
	a.InvalidateAll()
}

func (a *App) consumePendingToast(c *ui.Context) {
	a.mu.Lock()
	msg := a.pendingToast
	undo := a.pendingUndo
	a.pendingToast = ""
	a.pendingUndo = nil
	a.mu.Unlock()
	if msg == "" {
		return
	}
	if undo != nil {
		c.ToastAction(msg, "撤销", undo)
	} else {
		c.Toast(msg)
	}
}

func (a *App) InvalidateAll() {
	if a.MainWin != nil && !a.MainWin.IsDestroyed() {
		a.MainWin.Invalidate()
	}
	if a.TimerWin != nil && !a.TimerWin.IsDestroyed() {
		a.TimerWin.Invalidate()
	}
}

func (a *App) paletteOf(c *ui.Context) (*ui.Theme, Palette) {
	a.mu.Lock()
	sysDark := a.sysDark
	hasSys := a.hasSysDark
	a.mu.Unlock()
	if !hasSys {
		sysDark = c.Theme().Dark
	}
	mode := a.Timer.Prefs().Theme
	dark := ResolveDark(mode, sysDark)
	return BuildUITheme(dark)
}

func (a *App) windowBgCSS() string {
	a.mu.Lock()
	sysDark := a.sysDark
	hasSys := a.hasSysDark
	a.mu.Unlock()
	if !hasSys && mygo.App.IsReady() {
		sysDark = mygo.Theme.IsDark()
	}
	if ResolveDark(a.Timer.Prefs().Theme, sysDark) {
		return "#0D141E"
	}
	return "#F3F7FC"
}

func (a *App) OpenMainWindow() {
	if a.MainWin != nil && !a.MainWin.IsDestroyed() {
		if a.MainWin.IsMinimized() {
			a.MainWin.Restore()
		}
		if !a.MainWin.IsVisible() {
			a.MainWin.Show()
		}
		a.MainWin.Focus()
		return
	}

	opts := mygo.WindowOptions{
		Title:           "526Hz",
		Width:           1060,
		Height:          780,
		MinWidth:        720,
		MinHeight:       560,
		BackgroundColor: a.windowBgCSS(),
		StateKey:        "main",
		TitleBarStyle:   mygo.TitleBarHidden,
		Content:         ui.View(a.MainView),
	}
	if runtime.GOOS == "darwin" {
		opts.TrafficLightPosition = &mygo.Point{X: 18, Y: 20}
	} else {
		opts.HideWindowButtons = true
	}
	w := mygo.NewWindow(opts)
	_ = w.SetIcon(assets.AppIconPNG)
	w.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting {
			return
		}
		prefs := a.Timer.Prefs()
		if prefs.CloseToTray && a.Tray != nil && a.Tray.Exists() {
			e.PreventDefault()
			w.Hide()
			if !a.trayHinted {
				a.trayHinted = true
				body := "专注钟继续计时。右键任务栏托盘图标可开始/暂停或退出。"
				if runtime.GOOS == "darwin" {
					body = "专注钟继续计时。点菜单栏图标可回到主窗，右键可开始/暂停或退出。"
				}
				a.SendNotification("526Hz 仍在后台运行", body)
			}
			a.Log.Info("主窗已隐藏到托盘")
		}
	})
	w.OnClosed(func() {
		a.MainWin = nil
	})
	a.MainWin = w
}

func (a *App) OpenTimerWindow() {
	if a.TimerWin != nil && !a.TimerWin.IsDestroyed() {
		if !a.TimerWin.IsVisible() {
			a.TimerWin.Show()
		}
		a.TimerWin.Focus()
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{
		Title:             "专注钟",
		Width:             320,
		Height:            430,
		MinWidth:          210,
		MinHeight:         48,
		Frameless:         true,
		CornerRadius:      14,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		DisableMaximize:   true,
		DisableMinimize:   true,
		DisableFullScreen: true,
		BackgroundColor:   a.windowBgCSS(),
		StateKey:          "timer-compact",
		Content:           ui.View(a.TimerView),
	})
	_ = w.SetIcon(assets.AppIconPNG)
	if runtime.GOOS == "darwin" {
		w.SetVisibleOnAllWorkspaces(true)
	}
	w.OnClosed(func() {
		a.TimerWin = nil
		a.timerMiniPill = false
	})
	a.TimerWin = w
}

// SetTimerMiniPill toggles the compact timer between full ocean card (320x430) and floating mini pill (264x50).
func (a *App) SetTimerMiniPill(mini bool) {
	a.timerMiniPill = mini
	if mini {
		a.timerShowSetup = false
		a.timerShowSet = false
		a.timerDoneVisible = false
	}
	if a.TimerWin != nil && !a.TimerWin.IsDestroyed() {
		if mini {
			a.TimerWin.SetSize(264, 50)
		} else {
			a.TimerWin.SetSize(320, 430)
		}
		a.TimerWin.Invalidate()
	}
}

func (a *App) StartDesktop(compactOnly bool, startHidden bool) {
	a.mu.Lock()
	a.sysDark = mygo.Theme.IsDark()
	a.hasSysDark = true
	a.mu.Unlock()
	SyncNativeThemeMode(a.Timer.Prefs().Theme)
	if runtime.GOOS == "darwin" {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
	}

	a.Tray = NewTray(a)
	if a.Timer.Prefs().Tray || startHidden {
		a.Tray.Create()
	}

	a.Timer.StartTicker()
	a.Alarm.Start()
	a.startWeatherLoop()

	if compactOnly {
		a.OpenTimerWindow()
	} else if startHidden && a.Tray != nil && a.Tray.Exists() {
		a.Log.Info("已静默启动至系统托盘")
	} else {
		a.OpenMainWindow()
	}

	mygo.Theme.OnUpdated(func() {
		a.mu.Lock()
		a.sysDark = mygo.Theme.IsDark()
		a.hasSysDark = true
		a.mu.Unlock()
		a.InvalidateAll()
	})
	mygo.Power.OnSuspend(func() {
		a.Log.Info("系统休眠，暂停心跳")
		a.Timer.PauseBeforeSleep()
	})
	mygo.Power.OnResume(func() {
		a.Log.Info("系统唤醒，重算计时")
		a.Timer.ResumeAfterWake()
		a.Alarm.Tick()
	})
	mygo.Power.OnUnlockScreen(func() {
		a.Timer.Tick()
		a.Alarm.Tick()
	})
	mygo.App.OnActivate(func(hasVisible bool) {
		if !hasVisible {
			if compactOnly {
				a.OpenTimerWindow()
			} else {
				a.OpenMainWindow()
			}
		} else {
			a.OpenMainWindow()
		}
	})
	mygo.App.OnWindowAllClosed(func() {
		if runtime.GOOS == "darwin" {
			return
		}
		if a.Tray != nil && a.Tray.Exists() && !a.quitting {
			return
		}
		mygo.App.Quit()
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		a.quitting = true
		a.updateKeepRunning(false)
		a.stopWeatherLoop()
		if a.Ambient != nil {
			a.Ambient.Close()
		}
		a.Timer.StopTicker()
		a.Alarm.Stop()
		a.Timer.Flush()
		_ = a.Store.Flush()
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) {
		a.Timer.Flush()
		_ = a.Store.Flush()
		if a.Tray != nil {
			a.Tray.Destroy()
		}
		a.Log.Info("退出")
	})
}

// StartTaskFocus binds a task to the focus timer and enters the unified in-window Focus Stage.
const resetConfirmWindow = 4 * time.Second

// resetArmed reports whether the reset button is currently waiting for its confirming second click.
func (a *App) resetArmed() bool {
	return !a.resetArmedAt.IsZero() && time.Since(a.resetArmedAt) < resetConfirmWindow
}

// requestReset resets immediately when no round is in progress. An in-progress round is recorded as
// abandoned by a reset, so it needs a second click within resetConfirmWindow.
func (a *App) requestReset(snap timer.Snapshot) {
	if snap.Timer.StartedAt == "" || a.resetArmed() {
		a.resetArmedAt = time.Time{}
		a.Timer.Command("reset")
		return
	}
	a.resetArmedAt = time.Now()
	time.AfterFunc(resetConfirmWindow+50*time.Millisecond, a.InvalidateAll)
}

func (a *App) StartTaskFocus(c *ui.Context, t store.Task) {
	// One click starts the round with the current duration/plan; the plan can still be changed from the Focus Clock.
	res := a.Timer.StartTask(t.ID, a.Todo.ViewDay(), false)
	if !res.OK {
		c.Toast(res.Error)
		return
	}
	a.timerShowSetup = false
	a.bindPickOpen = false
	a.timerDoneVisible = false
	a.activeOverlay = "focus"
	if a.TimerWin != nil && !a.TimerWin.IsDestroyed() {
		a.TimerWin.Invalidate()
	}
}

func (a *App) LaunchImportPicker(c *ui.Context) {
	parent := a.MainWin
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  parent,
			Title:   "选择 526Hz 备份文件",
			Filters: []mygo.FileFilter{{Name: "JSON 备份", Extensions: []string{"json"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		raw, rerr := os.ReadFile(paths[0])
		if rerr != nil {
			a.SetPendingToast("读取备份文件失败", nil)
			return
		}
		val := store.ValidateBundle(raw)
		if !val.OK {
			msg := strings.Join(val.Errors, "；")
			if len([]rune(msg)) > 120 {
				msg = string([]rune(msg)[:120])
			}
			a.SetPendingToast("备份校验失败："+msg, nil)
			return
		}
		runOnUI(parent, func() {
			a.importBundle = raw
			a.importSummary = val.Summary
			a.importOpen = true
		})
	}()
}

func (a *App) LaunchExportDialog() {
	parent := a.MainWin
	go func() {
		bundle := a.Store.ExportAll()
		raw, err := marshalPretty(bundle)
		if err != nil {
			a.SetPendingToast("导出序列化失败", nil)
			return
		}
		docsDir, _ := mygo.App.Path(mygo.PathDocuments)
		defPath := filepath.Join(docsDir, fmt.Sprintf("526Hz-backup-%s.json", a.now().Format("2006-01-02")))
		dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      parent,
			Title:       "导出 526Hz 备份",
			DefaultPath: defPath,
			Filters:     []mygo.FileFilter{{Name: "JSON 文件", Extensions: []string{"json"}}},
		})
		if err != nil || dest == "" {
			return
		}
		if werr := os.WriteFile(dest, raw, 0o644); werr != nil {
			a.SetPendingToast("导出保存失败："+werr.Error(), nil)
			return
		}
		a.Log.Info("已导出备份到", dest)
		a.SetPendingToast("已导出备份文件", nil)
	}()
}

func (a *App) SaveNoteToFile(req notes.BuildRequest) {
	res := notes.BuildNote(a.Store, req, a.now())
	if !res.OK {
		a.SetPendingToast(res.Error, nil)
		return
	}
	parent := a.MainWin
	go func() {
		docsDir, _ := mygo.App.Path(mygo.PathDocuments)
		dest, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      parent,
			Title:       "导出笔记（Markdown）",
			DefaultPath: filepath.Join(docsDir, res.FileName),
			Filters:     []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md"}}},
		})
		if err != nil || dest == "" {
			return
		}
		if werr := os.WriteFile(dest, []byte(res.Text), 0o644); werr != nil {
			a.SetPendingToast("导出笔记失败："+werr.Error(), nil)
			return
		}
		a.Log.Info("已导出笔记到", dest)
		a.SetPendingToast("已保存笔记："+filepath.Base(dest), nil)
	}()
}

func runOnUI(w *mygo.Window, fn func()) {
	if w != nil && !w.IsDestroyed() {
		w.Update(fn)
		return
	}
	if mygo.App.IsReady() {
		mygo.RunOnMain(fn)
	} else {
		fn()
	}
}
