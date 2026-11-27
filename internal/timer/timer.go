package timer

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/store"
)

const (
	TickInterval   = 250 * time.Millisecond
	PersistEvery   = 5 * time.Second
	ClockJumpLimit = 5 * time.Second
)

type PlanPreset struct {
	Key    string
	Name   string
	Rounds int
	Focus  int
	Short  int
	Long   int
}

var DefaultPlans = []PlanPreset{
	{Key: "classic", Name: "标准番茄", Rounds: 4, Focus: 25, Short: 5, Long: 15},
	{Key: "deep", Name: "深专注", Rounds: 2, Focus: 45, Short: 5, Long: 15},
	{Key: "sprint", Name: "短冲刺", Rounds: 4, Focus: 15, Short: 3, Long: 10},
	{Key: "marathon", Name: "长跑", Rounds: 2, Focus: 50, Short: 10, Long: 20},
}

func ExpandPlan(def PlanPreset) []store.PlanStep {
	steps := make([]store.PlanStep, 0, def.Rounds*2)
	for i := 0; i < def.Rounds; i++ {
		steps = append(steps, store.PlanStep{Phase: "focus", Sec: def.Focus * 60})
		if i < def.Rounds-1 {
			steps = append(steps, store.PlanStep{Phase: "short", Sec: def.Short * 60})
		}
	}
	if def.Long > 0 {
		steps = append(steps, store.PlanStep{Phase: "long", Sec: def.Long * 60})
	}
	return steps
}

func PhaseLabel(phase string) string {
	switch phase {
	case "short":
		return "短休息"
	case "long":
		return "长休息"
	default:
		return "专注"
	}
}

type Prefs struct {
	Sound        bool   `json:"sound"`
	Notify       bool   `json:"notify"`
	Motion       string `json:"motion"` // full | calm | off
	Theme        string `json:"theme"`  // light | dark | system
	Tray         bool   `json:"tray"`
	CloseToTray  bool   `json:"closeToTray"`
	HighContrast bool   `json:"highContrast"`
	ShowSeconds  bool   `json:"showSeconds"`
	Phases       Phases `json:"phases"`
}

type Phases struct {
	Focus int `json:"focus"`
	Short int `json:"short"`
	Long  int `json:"long"`
}

func DefaultPrefs() Prefs {
	return Prefs{
		Sound:       true,
		Notify:      true,
		Motion:      "full",
		Theme:       "light",
		Tray:        true,
		CloseToTray: true,
		ShowSeconds: true,
		Phases:      Phases{Focus: 25 * 60, Short: 5 * 60, Long: 15 * 60},
	}
}

type PrefsPatch struct {
	Sound        *bool   `json:"sound,omitempty"`
	Notify       *bool   `json:"notify,omitempty"`
	Motion       *string `json:"motion,omitempty"`
	Theme        *string `json:"theme,omitempty"`
	Tray         *bool   `json:"tray,omitempty"`
	CloseToTray  *bool   `json:"closeToTray,omitempty"`
	HighContrast *bool   `json:"highContrast,omitempty"`
	ShowSeconds  *bool   `json:"showSeconds,omitempty"`
	FocusSec     *int    `json:"focusSec,omitempty"`
	ShortSec     *int    `json:"shortSec,omitempty"`
	LongSec      *int    `json:"longSec,omitempty"`
}

type Rejection struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
	At      string `json:"at"`
	From    string `json:"from"`
}

type PlanView struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
	Rounds int    `json:"rounds"`
}

type TimerView struct {
	Phase      string          `json:"phase"`
	PhaseLabel string          `json:"phaseLabel"`
	TotalSec   int             `json:"totalSec"`
	LeftSec    int             `json:"leftSec"`
	LeftMs     int64           `json:"leftMs"`
	Running    bool            `json:"running"`
	StartedAt  string          `json:"startedAt"`
	PauseCount int             `json:"pauseCount"`
	PausedMs   int64           `json:"pausedMs"`
	Paused     bool            `json:"paused"`
	Idle       bool            `json:"idle"`
	Round      int             `json:"round"`
	Abandoned  bool            `json:"abandoned"`
	Recovered  bool            `json:"recovered"`
	Task       *store.TaskRef  `json:"task"`
	AwaitPlan  bool            `json:"awaitPlan"`
	Plan       *PlanView       `json:"plan"`
	Progress   float64         `json:"progress"`
}

type Snapshot struct {
	Revision      int              `json:"revision"`
	ServerNow     int64            `json:"serverNow"`
	ClockJumpAt   int64            `json:"clockJumpAt"`
	SMState       string           `json:"smState"`
	LastRejection *Rejection       `json:"lastRejection"`
	Phases        Phases           `json:"phases"`
	Prefs         Prefs            `json:"prefs"`
	Stats         store.TimerStats `json:"stats"`
	Timer         TimerView        `json:"timer"`
}

type Event struct {
	Type           string `json:"type"` // done | abandoned | plan-advance | plan-set | phase-changed | day-rollover | await-plan
	Phase          string `json:"phase,omitempty"`
	PrevPhase      string `json:"prevPhase,omitempty"`
	Silent         bool   `json:"silent,omitempty"`
	Minutes        int    `json:"minutes,omitempty"`
	PlannedMinutes int    `json:"plannedMinutes,omitempty"`
	ActualMinutes  int    `json:"actualMinutes,omitempty"`
	ActualMs       int64  `json:"actualMs,omitempty"`
	Round          int    `json:"round,omitempty"`
	Rounds         int    `json:"rounds,omitempty"`
	MinutesToday   int    `json:"minutesToday,omitempty"`
	PauseCount     int    `json:"pauseCount,omitempty"`
	PausedMs       int64  `json:"pausedMs,omitempty"`
	StartedAt      string `json:"startedAt,omitempty"`
	FinishedAt     string `json:"finishedAt,omitempty"`
	AbandonedAt    string `json:"abandonedAt,omitempty"`
	Day            string `json:"day,omitempty"`
	Prev           string `json:"prev,omitempty"`
	TaskID         string `json:"taskId,omitempty"`
	TaskTitle      string `json:"taskTitle,omitempty"`
	Name           string `json:"name,omitempty"`
	Key            string `json:"key,omitempty"`
	Index          int    `json:"index,omitempty"`
	Total          int    `json:"total,omitempty"`
	TotalSec       int    `json:"totalSec,omitempty"`
	Steps          int    `json:"steps,omitempty"`
}

type StartTaskResult struct {
	OK       bool
	Code     string
	Error    string
	Snapshot Snapshot
}

type runtimeState struct {
	phase          string
	totalSec       int
	leftMs         int64
	running        bool
	endAtMs        int64
	startMonoMs    int64
	leftAtStartMs  int64
	startedAt      string
	pauseCount     int
	pausedMs       int64
	pauseStartedAt int64
	round          int
	abandoned      bool
	recovered      bool
	task           *store.TaskRef
	plan           *store.PlanState
	awaitPlan      bool
}

// Engine is the single authoritative timer state machine.
type Engine struct {
	mu          sync.Mutex
	store       *store.Store
	log         *store.Logger
	now         func() time.Time
	mono        func() time.Duration
	onBroadcast func(Snapshot)
	onEvent     func(Event)
	notify      func(title, body string)

	prefs         Prefs
	stats         store.TimerStats
	st            *runtimeState
	revision      int
	lastPersist   int64
	lastWallMs    int64
	lastMonoMs    int64
	clockJumpAt   int64
	lastRejection *Rejection
	finishing     bool
	abandoning    bool
	rejecting     bool

	tickerMu sync.Mutex
	ticker   *time.Ticker
	stopCh   chan struct{}
}

type Options struct {
	Store       *store.Store
	Log         *store.Logger
	Now         func() time.Time
	Mono        func() time.Duration
	OnBroadcast func(Snapshot)
	OnEvent     func(Event)
	Notify      func(title, body string)
}

func NewEngine(opts Options) *Engine {
	boot := time.Now()
	nowFn := opts.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	monoFn := opts.Mono
	if monoFn == nil {
		monoFn = func() time.Duration { return time.Since(boot) }
	}
	e := &Engine{
		store:       opts.Store,
		log:         opts.Log,
		now:         nowFn,
		mono:        monoFn,
		onBroadcast: opts.OnBroadcast,
		onEvent:     opts.OnEvent,
		notify:      opts.Notify,
		prefs:       DefaultPrefs(),
	}
	return e
}

func (e *Engine) nowMs() int64  { return e.now().UnixMilli() }
func (e *Engine) monoMs() int64 { return e.mono().Milliseconds() }
func (e *Engine) todayKey() string {
	return dateutil.TodayKey(e.now())
}

func (e *Engine) Init() Snapshot {
	e.mu.Lock()
	e.loadPrefsLocked()
	e.loadStatsLocked()
	e.loadStateLocked()
	e.store.Remove("sisy-active-task")
	e.lastWallMs = e.nowMs()
	e.lastMonoMs = e.monoMs()
	snap, cb := e.prepareBroadcastLocked()
	e.mu.Unlock()
	if cb != nil {
		cb(snap)
	}
	return snap
}

func (e *Engine) StartTicker() {
	e.tickerMu.Lock()
	defer e.tickerMu.Unlock()
	if e.ticker != nil {
		return
	}
	t := time.NewTicker(TickInterval)
	stop := make(chan struct{})
	e.ticker = t
	e.stopCh = stop
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				e.Tick()
			}
		}
	}()
}

func (e *Engine) StopTicker() {
	e.tickerMu.Lock()
	defer e.tickerMu.Unlock()
	if e.stopCh != nil {
		close(e.stopCh)
		e.stopCh = nil
	}
	if e.ticker != nil {
		e.ticker.Stop()
		e.ticker = nil
	}
}

func (e *Engine) Flush() {
	e.mu.Lock()
	if e.st != nil {
		e.persistLocked(true)
	}
	e.mu.Unlock()
	_ = e.store.Flush()
}

func (e *Engine) Snapshot() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked()
}

func (e *Engine) Prefs() Prefs {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.prefs
}

func (e *Engine) History() []store.HistoryEntry {
	var list []store.HistoryEntry
	if e.store.GetJSON(store.KeyTimerHistory, &list) && list != nil {
		return list
	}
	return []store.HistoryEntry{}
}

// DeleteHistoryByIndices permanently removes specific history entries, recalculates today's stats, and returns removed items for undo.
func (e *Engine) DeleteHistoryByIndices(indices []int) []store.HistoryEntry {
	if len(indices) == 0 {
		return nil
	}
	drop := make(map[int]bool, len(indices))
	for _, idx := range indices {
		drop[idx] = true
	}

	e.mu.Lock()
	var list []store.HistoryEntry
	_ = e.store.GetJSON(store.KeyTimerHistory, &list)
	kept := make([]store.HistoryEntry, 0, len(list))
	removed := make([]store.HistoryEntry, 0, len(indices))
	for i, item := range list {
		if drop[i] {
			removed = append(removed, item)
		} else {
			kept = append(kept, item)
		}
	}
	if len(removed) == 0 {
		e.mu.Unlock()
		return nil
	}
	_ = e.store.SetJSON(store.KeyTimerHistory, kept)
	e.recomputeTodayStatsFromHistoryLocked(kept)
	snap, bcb := e.prepareBroadcastLocked()
	e.mu.Unlock()

	if bcb != nil {
		bcb(snap)
	}
	return removed
}

// RestoreHistoryEntries re-appends previously deleted history entries (for undo) and recalculates today's stats.
func (e *Engine) RestoreHistoryEntries(entries []store.HistoryEntry) {
	if len(entries) == 0 {
		return
	}
	e.mu.Lock()
	var list []store.HistoryEntry
	_ = e.store.GetJSON(store.KeyTimerHistory, &list)
	list = append(list, entries...)
	if len(list) > store.HistoryMax {
		list = list[len(list)-store.HistoryMax:]
	}
	_ = e.store.SetJSON(store.KeyTimerHistory, list)
	e.recomputeTodayStatsFromHistoryLocked(list)
	snap, bcb := e.prepareBroadcastLocked()
	e.mu.Unlock()

	if bcb != nil {
		bcb(snap)
	}
}

func (e *Engine) recomputeTodayStatsFromHistoryLocked(list []store.HistoryEntry) {
	today := e.todayKey()
	rounds, minutes := 0, 0
	for _, h := range list {
		if h.Day == today && h.Phase == "focus" && (h.Result == "done" || h.Result == "recovered" || h.Result == "") {
			rounds++
			minutes += max(0, h.PlannedMinutes)
		}
	}
	e.stats = store.TimerStats{Day: today, Rounds: rounds, Minutes: minutes}
	e.persistStatsLocked()
	if !e.st.running {
		e.st.round = rounds + 1
		e.persistLocked(true)
	}
}

func (e *Engine) SetPrefs(patch PrefsPatch) Snapshot {
	e.mu.Lock()
	if patch.Sound != nil {
		e.prefs.Sound = *patch.Sound
	}
	if patch.Notify != nil {
		e.prefs.Notify = *patch.Notify
	}
	if patch.Tray != nil {
		e.prefs.Tray = *patch.Tray
	}
	if patch.CloseToTray != nil {
		e.prefs.CloseToTray = *patch.CloseToTray
	}
	if patch.HighContrast != nil {
		e.prefs.HighContrast = *patch.HighContrast
	}
	if patch.ShowSeconds != nil {
		e.prefs.ShowSeconds = *patch.ShowSeconds
	}
	if patch.Motion != nil && slices.Contains(store.MotionSet, *patch.Motion) {
		e.prefs.Motion = *patch.Motion
	}
	if patch.Theme != nil && slices.Contains(store.ThemeSet, *patch.Theme) {
		e.prefs.Theme = *patch.Theme
	}
	if patch.FocusSec != nil {
		e.prefs.Phases.Focus = store.ClampInt(*patch.FocusSec, 60, 180*60)
	}
	if patch.ShortSec != nil {
		e.prefs.Phases.Short = store.ClampInt(*patch.ShortSec, 60, 180*60)
	}
	if patch.LongSec != nil {
		e.prefs.Phases.Long = store.ClampInt(*patch.LongSec, 60, 180*60)
	}
	e.savePrefsLocked()
	if e.st != nil && !e.st.running && e.st.startedAt == "" {
		if d := e.phaseDurationLocked(e.st.phase); d > 0 {
			e.st.totalSec = d
			e.st.leftMs = int64(d) * 1000
			e.persistLocked(true)
		}
	}
	snap, cb := e.prepareBroadcastLocked()
	e.mu.Unlock()
	if cb != nil {
		cb(snap)
	}
	return snap
}

func (e *Engine) ReloadFromStore() Snapshot {
	return e.Init()
}

func (e *Engine) phaseDurationLocked(phase string) int {
	switch phase {
	case "short":
		return e.prefs.Phases.Short
	case "long":
		return e.prefs.Phases.Long
	default:
		return e.prefs.Phases.Focus
	}
}

func (e *Engine) loadPrefsLocked() {
	e.prefs = DefaultPrefs()
	var raw store.TimerPrefs
	if !e.store.GetJSON(store.KeyTimerPrefs, &raw) {
		return
	}
	if raw.Sound != nil {
		e.prefs.Sound = *raw.Sound
	}
	if raw.Notify != nil {
		e.prefs.Notify = *raw.Notify
	}
	if raw.Tray != nil {
		e.prefs.Tray = *raw.Tray
	}
	if raw.CloseToTray != nil {
		e.prefs.CloseToTray = *raw.CloseToTray
	}
	if raw.HighContrast != nil {
		e.prefs.HighContrast = *raw.HighContrast
	}
	if raw.ShowSeconds != nil {
		e.prefs.ShowSeconds = *raw.ShowSeconds
	}
	if slices.Contains(store.MotionSet, raw.Motion) {
		e.prefs.Motion = raw.Motion
	}
	if slices.Contains(store.ThemeSet, raw.Theme) {
		e.prefs.Theme = raw.Theme
	}
	if raw.Phases != nil {
		if raw.Phases.Focus > 0 {
			e.prefs.Phases.Focus = store.ClampInt(raw.Phases.Focus, 60, 180*60)
		}
		if raw.Phases.Short > 0 {
			e.prefs.Phases.Short = store.ClampInt(raw.Phases.Short, 60, 180*60)
		}
		if raw.Phases.Long > 0 {
			e.prefs.Phases.Long = store.ClampInt(raw.Phases.Long, 60, 180*60)
		}
	}
}

func (e *Engine) savePrefsLocked() {
	b := func(v bool) *bool { return &v }
	p := store.TimerPrefs{
		Sound:        b(e.prefs.Sound),
		Notify:       b(e.prefs.Notify),
		Motion:       e.prefs.Motion,
		Theme:        e.prefs.Theme,
		Tray:         b(e.prefs.Tray),
		CloseToTray:  b(e.prefs.CloseToTray),
		HighContrast: b(e.prefs.HighContrast),
		ShowSeconds:  b(e.prefs.ShowSeconds),
		Phases: &store.PhaseDurations{
			Focus: e.prefs.Phases.Focus,
			Short: e.prefs.Phases.Short,
			Long:  e.prefs.Phases.Long,
		},
	}
	if r := e.store.SetJSON(store.KeyTimerPrefs, p); !r.OK && e.log != nil {
		e.log.Warn("保存偏好失败", r.Error)
	}
}

func (e *Engine) loadStatsLocked() {
	today := e.todayKey()
	var raw store.TimerStats
	if e.store.GetJSON(store.KeyTimerStats, &raw) && raw.Day != "" {
		nk := dateutil.NormalizeKey(raw.Day)
		if nk == "" {
			nk = today
		}
		e.stats = store.TimerStats{
			Day:     nk,
			Rounds:  max(0, raw.Rounds),
			Minutes: max(0, raw.Minutes),
		}
		if nk != raw.Day {
			if e.log != nil {
				e.log.Info("统计日期键规范化", raw.Day, "->", nk)
			}
			e.persistStatsLocked()
		}
	} else {
		e.stats = store.TimerStats{Day: today, Rounds: 0, Minutes: 0}
	}
	if e.stats.Day != today {
		e.stats = store.TimerStats{Day: today, Rounds: 0, Minutes: 0}
		e.persistStatsLocked()
	}
}

func (e *Engine) rollDayIfNeededLocked(silent bool, evs *[]Event) bool {
	today := e.todayKey()
	if e.stats.Day == today {
		return false
	}
	prev := e.stats.Day
	e.stats = store.TimerStats{Day: today, Rounds: 0, Minutes: 0}
	if !silent {
		if e.log != nil {
			e.log.Info(fmt.Sprintf("跨天，专注统计已重置（%s -> %s）", prev, today))
		}
		*evs = append(*evs, Event{Type: "day-rollover", Day: today, Prev: prev})
	}
	e.persistStatsLocked()
	if e.st != nil {
		e.persistLocked(true)
	}
	return true
}

func (e *Engine) newTimerLocked(phase string) *runtimeState {
	if !slices.Contains(store.PhaseSet, phase) {
		phase = "focus"
	}
	totalSec := e.phaseDurationLocked(phase)
	return &runtimeState{
		phase:         phase,
		totalSec:      totalSec,
		leftMs:        int64(totalSec) * 1000,
		leftAtStartMs: int64(totalSec) * 1000,
		round:         e.stats.Rounds + 1,
	}
}

func (e *Engine) loadStateLocked() {
	var raw store.PersistedTimerState
	if !e.store.GetJSON(store.KeyTimerState, &raw) || !slices.Contains(store.PhaseSet, raw.Phase) {
		e.st = e.newTimerLocked("focus")
		return
	}
	t := e.newTimerLocked(raw.Phase)
	if raw.TotalSec > 0 {
		t.totalSec = store.ClampInt(raw.TotalSec, 1, 180*60)
	}
	t.pauseCount = max(0, raw.PauseCount)
	t.pausedMs = max(0, raw.PausedMs)
	if raw.StartedAt != nil {
		t.startedAt = *raw.StartedAt
	}
	if raw.Phase == "focus" && t.startedAt != "" && raw.Task != nil {
		cp := *raw.Task
		t.task = &cp
	}
	if raw.Plan != nil && len(raw.Plan.Steps) > 0 {
		steps := make([]store.PlanStep, len(raw.Plan.Steps))
		copy(steps, raw.Plan.Steps)
		t.plan = &store.PlanState{
			Key:   raw.Plan.Key,
			Name:  raw.Plan.Name,
			Steps: steps,
			Index: store.ClampInt(raw.Plan.Index, 0, len(steps)-1),
		}
	}
	t.abandoned = raw.Abandoned
	t.round = max(1, raw.Round)
	if !raw.Running && t.startedAt != "" && raw.PauseStartedAt > 0 {
		nowMs := e.nowMs()
		if raw.PauseStartedAt > nowMs {
			t.pauseStartedAt = nowMs
		} else {
			t.pauseStartedAt = raw.PauseStartedAt
		}
	}
	if raw.Running && raw.EndAt > 0 {
		remainMs := raw.EndAt - e.nowMs()
		if remainMs <= 0 {
			t.running = false
			t.leftMs = 0
			e.st = t
			var evs []Event
			var notes [][2]string
			e.completeLocked(true, &evs, &notes)
			// Fire silent recovered event asynchronously via caller? Init doesn't suppress silent done event, fire directly
			for _, ev := range evs {
				if e.onEvent != nil {
					e.onEvent(ev)
				}
			}
			return
		}
		t.running = true
		t.endAtMs = raw.EndAt
		t.leftMs = min(remainMs, int64(t.totalSec)*1000)
		t.startMonoMs = e.monoMs()
		t.leftAtStartMs = t.leftMs
		t.recovered = true
		if e.log != nil {
			e.log.Info(fmt.Sprintf("恢复运行中的计时：剩余 %ds", (t.leftMs+500)/1000))
		}
	} else {
		t.running = false
		t.leftMs = min(max(0, raw.LeftMs), int64(t.totalSec)*1000)
		if t.leftMs <= 0 {
			t.leftMs = int64(t.totalSec) * 1000
		}
	}
	e.st = t
}

func (e *Engine) persistLocked(force bool) {
	t0 := e.nowMs()
	if !force && t0-e.lastPersist < PersistEvery.Milliseconds() {
		return
	}
	e.lastPersist = t0
	t := e.st
	var startedPtr *string
	if t.startedAt != "" {
		s := t.startedAt
		startedPtr = &s
	}
	var endAt int64
	if t.running {
		endAt = t.endAtMs
	}
	ps := store.PersistedTimerState{
		Phase:          t.phase,
		TotalSec:       t.totalSec,
		LeftMs:         t.leftMs,
		Running:        t.running,
		EndAt:          endAt,
		StartedAt:      startedPtr,
		PauseCount:     t.pauseCount,
		PausedMs:       t.pausedMs,
		PauseStartedAt: t.pauseStartedAt,
		Round:          t.round,
		Abandoned:      t.abandoned,
		UpdatedAt:      e.now().UTC().Format(time.RFC3339Nano),
		Task:           t.task,
		Plan:           t.plan,
	}
	if r := e.store.SetJSON(store.KeyTimerState, ps); !r.OK && e.log != nil {
		e.log.Warn("保存计时状态失败", r.Error)
	}
	if t.running {
		e.store.SetJSON(store.KeyTimerRun, map[string]any{"totalSec": t.totalSec, "running": true, "endAt": t.endAtMs})
	} else {
		e.store.SetJSON(store.KeyTimerRun, map[string]any{"totalSec": t.totalSec, "running": false, "leftSec": (t.leftMs + 500) / 1000})
	}
}

func (e *Engine) persistStatsLocked() {
	st := store.TimerStats{
		Day:       e.stats.Day,
		Rounds:    e.stats.Rounds,
		Minutes:   e.stats.Minutes,
		UpdatedAt: e.now().UTC().Format(time.RFC3339Nano),
	}
	if r := e.store.SetJSON(store.KeyTimerStats, st); !r.OK && e.log != nil {
		e.log.Warn("保存统计失败", r.Error)
	}
}

func (e *Engine) appendHistoryLocked(entry store.HistoryEntry) {
	var list []store.HistoryEntry
	_ = e.store.GetJSON(store.KeyTimerHistory, &list)
	list = append(list, entry)
	if len(list) > store.HistoryMax {
		list = list[len(list)-store.HistoryMax:]
	}
	if r := e.store.SetJSON(store.KeyTimerHistory, list); !r.OK && e.log != nil {
		e.log.Warn("保存专注历史失败", r.Error)
	}
}

func (e *Engine) currentLeftMsLocked() int64 {
	t := e.st
	if !t.running {
		return max(0, t.leftMs)
	}
	left := t.leftAtStartMs - (e.monoMs() - t.startMonoMs)
	if left < -60_000 {
		left = t.endAtMs - e.nowMs()
	}
	return min(max(0, left), int64(t.totalSec)*1000)
}

func (e *Engine) smStateLocked() string {
	if e.st == nil {
		return "none"
	}
	if e.st.running {
		return "running"
	}
	if e.st.startedAt != "" {
		return "paused"
	}
	return "idle"
}

func (e *Engine) snapshotLocked() Snapshot {
	t := e.st
	leftMs := e.currentLeftMsLocked()
	leftSec := int((leftMs + 999) / 1000)
	var progress float64
	if t.totalSec > 0 {
		progress = 1.0 - float64(leftMs)/float64(int64(t.totalSec)*1000)
		if progress < 0 {
			progress = 0
		} else if progress > 1 {
			progress = 1
		}
	}
	var rej *Rejection
	if e.lastRejection != nil {
		cp := *e.lastRejection
		rej = &cp
	}
	var taskCopy *store.TaskRef
	if t.task != nil {
		cp := *t.task
		taskCopy = &cp
	}
	var planView *PlanView
	if t.plan != nil {
		rounds := 0
		for _, s := range t.plan.Steps {
			if s.Phase == "focus" {
				rounds++
			}
		}
		planView = &PlanView{
			Name:   t.plan.Name,
			Key:    t.plan.Key,
			Index:  t.plan.Index,
			Total:  len(t.plan.Steps),
			Rounds: rounds,
		}
	}
	return Snapshot{
		Revision:      e.revision,
		ServerNow:     e.nowMs(),
		ClockJumpAt:   e.clockJumpAt,
		SMState:       e.smStateLocked(),
		LastRejection: rej,
		Phases:        e.prefs.Phases,
		Prefs:         e.prefs,
		Stats:         e.stats,
		Timer: TimerView{
			Phase:      t.phase,
			PhaseLabel: PhaseLabel(t.phase),
			TotalSec:   t.totalSec,
			LeftSec:    leftSec,
			LeftMs:     leftMs,
			Running:    t.running,
			StartedAt:  t.startedAt,
			PauseCount: t.pauseCount,
			PausedMs:   t.pausedMs,
			Paused:     !t.running && t.startedAt != "",
			Idle:       !t.running && t.startedAt == "",
			Round:      t.round,
			Abandoned:  t.abandoned,
			Recovered:  t.recovered,
			Task:       taskCopy,
			AwaitPlan:  t.awaitPlan,
			Plan:       planView,
			Progress:   progress,
		},
	}
}

func (e *Engine) prepareBroadcastLocked() (Snapshot, func(Snapshot)) {
	e.revision++
	return e.snapshotLocked(), e.onBroadcast
}

func (e *Engine) rejectLocked(cmd, reason string) bool {
	if len(cmd) > 32 {
		cmd = cmd[:32]
	}
	if len(reason) > 120 {
		reason = reason[:120]
	}
	e.lastRejection = &Rejection{
		Command: cmd,
		Reason:  reason,
		At:      e.now().UTC().Format(time.RFC3339Nano),
		From:    e.smStateLocked(),
	}
	if e.log != nil {
		e.log.Warn(fmt.Sprintf("拒绝非法状态转换 %s @ %s：%s", cmd, e.lastRejection.From, reason))
	}
	return false
}

var transitions = map[string][]string{
	"start":       {"idle", "paused"},
	"pause":       {"running"},
	"toggle":      {"idle", "running", "paused"},
	"reset":       {"idle", "running", "paused"},
	"setPhase":    {"idle", "running", "paused"},
	"setDuration": {"idle", "running", "paused"},
	"applyPlan":   {"idle", "running", "paused"},
	"abandon":     {"running", "paused"},
	"resetToday":  {"idle", "running", "paused"},
	"refresh":     {"idle", "running", "paused"},
}

func (e *Engine) canRunLocked(cmd string) bool {
	allow, ok := transitions[cmd]
	if !ok {
		return e.rejectLocked(cmd, "未知命令")
	}
	cur := e.smStateLocked()
	if !slices.Contains(allow, cur) {
		return e.rejectLocked(cmd, fmt.Sprintf("当前状态=%s", cur))
	}
	return true
}

func (e *Engine) setRunningLocked(on bool) {
	t := e.st
	if on == t.running {
		return
	}
	if on {
		t.awaitPlan = false
		if t.startedAt == "" {
			t.startedAt = e.now().UTC().Format(time.RFC3339Nano)
		}
		if t.leftMs <= 0 {
			t.leftMs = int64(t.totalSec) * 1000
		}
		t.startMonoMs = e.monoMs()
		t.leftAtStartMs = t.leftMs
		t.endAtMs = e.nowMs() + t.leftMs
		t.running = true
		if t.pauseStartedAt > 0 {
			e.st.pausedMs += max(0, e.nowMs()-t.pauseStartedAt)
			t.pauseStartedAt = 0
		}
		t.recovered = false
	} else {
		t.leftMs = e.currentLeftMsLocked()
		t.running = false
		t.endAtMs = 0
		if t.startedAt != "" {
			t.pauseCount++
			t.pauseStartedAt = e.nowMs()
		}
	}
	e.persistLocked(true)
}

func (e *Engine) completeLocked(silent bool, evs *[]Event, notes *[][2]string) {
	if e.finishing {
		e.rejectLocked("complete", "重入被拦截")
		return
	}
	e.finishing = true
	defer func() { e.finishing = false }()

	t := e.st
	finishedAt := e.now()
	t.running = false
	t.endAtMs = 0
	if t.pauseStartedAt > 0 {
		t.pausedMs += max(0, finishedAt.UnixMilli()-t.pauseStartedAt)
		t.pauseStartedAt = 0
	}

	plannedSec := t.totalSec
	actualMs := int64(plannedSec) * 1000
	if t.startedAt != "" {
		if stTime, err := time.Parse(time.RFC3339Nano, t.startedAt); err == nil {
			actualMs = max(0, finishedAt.Sub(stTime).Milliseconds())
		}
	}
	phase := t.phase
	activeTask := t.task
	credited := 0
	if phase == "focus" {
		e.rollDayIfNeededLocked(true, evs)
		credited = (plannedSec + 30) / 60
		e.stats.Rounds++
		e.stats.Minutes += credited
		e.persistStatsLocked()
	}

	var taskID, taskDay, taskTitle string
	if activeTask != nil {
		taskID = activeTask.ID
		taskDay = activeTask.Day
		taskTitle = activeTask.Title
	}
	plannedMin := (plannedSec + 30) / 60
	actualMin := max(1, int((actualMs+30_000)/60_000))
	finishedISO := finishedAt.UTC().Format(time.RFC3339Nano)

	payload := Event{
		Type:           "done",
		Phase:          phase,
		Silent:         silent,
		Minutes:        credited,
		PlannedMinutes: plannedMin,
		ActualMinutes:  actualMin,
		ActualMs:       actualMs,
		Round:          e.stats.Rounds,
		Rounds:         e.stats.Rounds,
		MinutesToday:   e.stats.Minutes,
		PauseCount:     t.pauseCount,
		PausedMs:       t.pausedMs,
		StartedAt:      t.startedAt,
		FinishedAt:     finishedISO,
		Day:            e.stats.Day,
		TaskID:         taskID,
		TaskTitle:      taskTitle,
	}

	plan := t.plan
	if !silent && plan != nil && plan.Index+1 < len(plan.Steps) {
		nextIdx := plan.Index + 1
		step := plan.Steps[nextIdx]
		nt := e.newTimerLocked(step.Phase)
		nt.totalSec = step.Sec
		nt.leftMs = int64(step.Sec) * 1000
		nt.plan = &store.PlanState{
			Key:   plan.Key,
			Name:  plan.Name,
			Steps: slices.Clone(plan.Steps),
			Index: nextIdx,
		}
		if activeTask != nil {
			cp := *activeTask
			nt.task = &cp
		}
		nt.round = e.stats.Rounds + 1
		nt.startedAt = e.now().UTC().Format(time.RFC3339Nano)
		nt.startMonoMs = e.monoMs()
		nt.leftAtStartMs = nt.leftMs
		nt.endAtMs = e.nowMs() + nt.leftMs
		nt.running = true
		e.st = nt
		e.persistLocked(true)
		e.appendHistoryLocked(store.HistoryEntry{
			Day:            payload.Day,
			Phase:          phase,
			StartedAt:      payload.StartedAt,
			FinishedAt:     finishedISO,
			PlannedMinutes: plannedMin,
			ActualMinutes:  actualMin,
			PauseCount:     payload.PauseCount,
			PausedMinutes:  int((payload.PausedMs + 30_000) / 60_000),
			Result:         "done",
			TaskID:         taskID,
			TaskDay:        taskDay,
			TaskTitle:      taskTitle,
		})
		*evs = append(*evs, Event{
			Type:      "plan-advance",
			Name:      plan.Name,
			Index:     nextIdx,
			Total:     len(plan.Steps),
			Phase:     step.Phase,
			PrevPhase: phase,
			Minutes:   (step.Sec + 30) / 60,
		})
		if e.prefs.Notify {
			title := "☕ 休息一下"
			kindText := "休息"
			if step.Phase == "focus" {
				title = "🔔 该专注了"
				kindText = "专注"
			}
			body := fmt.Sprintf("%s · %d/%d · %s %d 分钟", plan.Name, nextIdx+1, len(plan.Steps), kindText, (step.Sec+30)/60)
			*notes = append(*notes, [2]string{title, body})
		}
		if e.log != nil {
			e.log.Info(fmt.Sprintf("计划自动推进 %s %d/%d %s", plan.Name, nextIdx+1, len(plan.Steps), step.Phase))
		}
		return
	}

	e.st = e.newTimerLocked(phase)
	e.st.plan = nil
	e.st.round = e.stats.Rounds + 1
	e.persistLocked(true)
	resKind := "done"
	if silent {
		resKind = "recovered"
	}
	e.appendHistoryLocked(store.HistoryEntry{
		Day:            payload.Day,
		Phase:          phase,
		StartedAt:      payload.StartedAt,
		FinishedAt:     finishedISO,
		PlannedMinutes: plannedMin,
		ActualMinutes:  actualMin,
		PauseCount:     payload.PauseCount,
		PausedMinutes:  int((payload.PausedMs + 30_000) / 60_000),
		Result:         resKind,
		TaskID:         taskID,
		TaskDay:        taskDay,
		TaskTitle:      taskTitle,
	})
	*evs = append(*evs, payload)
	if !silent && e.prefs.Notify {
		var title, body string
		if phase == "focus" {
			title = "🎉 专注完成"
			body = fmt.Sprintf("本轮 %d 分钟 · 今日第 %d 轮 · 累计 %d 分钟", plannedMin, e.stats.Rounds, e.stats.Minutes)
		} else {
			title = "☕ 休息结束"
			body = fmt.Sprintf("该回到专注了 · 今日已专注 %d 分钟", e.stats.Minutes)
		}
		*notes = append(*notes, [2]string{title, body})
	}
	if e.log != nil {
		e.log.Info(fmt.Sprintf("阶段完成 %s 计划 %dmin 实际 %dmin 暂停 %d次", phase, plannedMin, actualMin, payload.PauseCount))
	}
}

func (e *Engine) abandonLocked(evs *[]Event) {
	if e.abandoning {
		e.rejectLocked("abandon", "重入被拦截")
		return
	}
	e.abandoning = true
	defer func() { e.abandoning = false }()

	t := e.st
	phase := t.phase
	activeTask := t.task
	plannedMin := (t.totalSec + 30) / 60
	var actualMs int64
	if t.startedAt != "" {
		if stTime, err := time.Parse(time.RFC3339Nano, t.startedAt); err == nil {
			actualMs = max(0, e.now().Sub(stTime).Milliseconds())
		}
	}
	abandonedISO := e.now().UTC().Format(time.RFC3339Nano)
	var taskID, taskDay, taskTitle string
	if activeTask != nil {
		taskID = activeTask.ID
		taskDay = activeTask.Day
		taskTitle = activeTask.Title
	}
	payload := Event{
		Type:           "abandoned",
		Phase:          phase,
		PlannedMinutes: plannedMin,
		ActualMinutes:  max(0, int((actualMs+30_000)/60_000)),
		PauseCount:     t.pauseCount,
		PausedMs:       t.pausedMs,
		StartedAt:      t.startedAt,
		AbandonedAt:    abandonedISO,
		TaskID:         taskID,
		TaskTitle:      taskTitle,
	}
	if e.log != nil {
		e.log.Info(fmt.Sprintf("放弃本轮 %s %dmin", phase, plannedMin))
	}
	e.st = e.newTimerLocked(phase)
	e.st.plan = nil
	e.st.round = e.stats.Rounds + 1
	e.persistLocked(true)
	e.appendHistoryLocked(store.HistoryEntry{
		Day:            e.todayKey(),
		Phase:          phase,
		StartedAt:      payload.StartedAt,
		FinishedAt:     abandonedISO,
		PlannedMinutes: plannedMin,
		ActualMinutes:  payload.ActualMinutes,
		PauseCount:     payload.PauseCount,
		PausedMinutes:  int((payload.PausedMs + 30_000) / 60_000),
		Result:         "abandoned",
		TaskID:         taskID,
		TaskDay:        taskDay,
		TaskTitle:      taskTitle,
	})
	*evs = append(*evs, payload)
}

type CmdPayload struct {
	Phase string
	Sec   int
	Plan  string
}

func (e *Engine) Command(cmd string, p ...CmdPayload) Snapshot {
	var pl CmdPayload
	if len(p) > 0 {
		pl = p[0]
	}

	e.mu.Lock()
	var evs []Event
	var notes [][2]string
	needBroadcast := false

	switch cmd {
	case "start":
		if e.canRunLocked("start") {
			e.setRunningLocked(true)
			needBroadcast = true
		}
	case "pause":
		if e.canRunLocked("pause") {
			e.setRunningLocked(false)
			needBroadcast = true
		}
	case "toggle":
		if e.canRunLocked("toggle") {
			e.setRunningLocked(!e.st.running)
			needBroadcast = true
		}
	case "reset":
		if e.canRunLocked("reset") {
			if e.st.startedAt != "" && !e.rejecting {
				e.rejecting = true
				e.abandonLocked(&evs)
				e.rejecting = false
			}
			f := e.st
			f.running = false
			f.endAtMs = 0
			f.leftMs = int64(f.totalSec) * 1000
			f.startedAt = ""
			f.task = nil
			f.pauseCount = 0
			f.pausedMs = 0
			f.pauseStartedAt = 0
			f.abandoned = false
			f.recovered = false
			f.plan = nil
			f.awaitPlan = false
			e.persistLocked(true)
			needBroadcast = true
		}
	case "setPhase":
		if e.canRunLocked("setPhase") {
			if !slices.Contains(store.PhaseSet, pl.Phase) {
				e.rejectLocked("setPhase", "未知阶段 "+pl.Phase)
			} else if e.st.phase != pl.Phase || e.st.startedAt != "" {
				if (e.st.running || e.st.startedAt != "") && !e.rejecting {
					e.rejecting = true
					e.abandonLocked(&evs)
					e.rejecting = false
				}
				e.st = e.newTimerLocked(pl.Phase)
				e.st.plan = nil
				e.st.round = e.stats.Rounds + 1
				e.persistLocked(true)
				needBroadcast = true
				evs = append(evs, Event{Type: "phase-changed", Phase: pl.Phase, TotalSec: e.st.totalSec})
			}
		}
	case "setDuration":
		if e.canRunLocked("setDuration") {
			if pl.Sec < 60 || pl.Sec > 180*60 {
				e.rejectLocked("setDuration", "sec 超出 60..10800 范围")
			} else {
				e.prefs.Phases.Focus = pl.Sec
				e.savePrefsLocked()
				bound := e.st.task
				if (e.st.running || e.st.startedAt != "") && !e.rejecting {
					e.rejecting = true
					e.abandonLocked(&evs)
					e.rejecting = false
				}
				e.st = e.newTimerLocked("focus")
				e.st.totalSec = pl.Sec
				e.st.leftMs = int64(pl.Sec) * 1000
				e.st.plan = nil
				e.st.task = bound
				e.st.awaitPlan = false
				e.st.round = e.stats.Rounds + 1
				e.persistLocked(true)
				needBroadcast = true
			}
		}
	case "applyPlan":
		if e.canRunLocked("applyPlan") {
			var def *PlanPreset
			for i := range DefaultPlans {
				if DefaultPlans[i].Key == pl.Plan {
					def = &DefaultPlans[i]
					break
				}
			}
			if def == nil {
				e.rejectLocked("applyPlan", "未知计划 "+pl.Plan)
			} else {
				if (e.st.running || e.st.startedAt != "") && !e.rejecting {
					e.rejecting = true
					e.abandonLocked(&evs)
					e.rejecting = false
				}
				bound := e.st.task
				steps := ExpandPlan(*def)
				e.st = e.newTimerLocked(steps[0].Phase)
				e.st.totalSec = steps[0].Sec
				e.st.leftMs = int64(steps[0].Sec) * 1000
				e.st.plan = &store.PlanState{Key: def.Key, Name: def.Name, Steps: steps, Index: 0}
				e.st.awaitPlan = false
				e.st.task = bound
				e.st.round = e.stats.Rounds + 1
				e.persistLocked(true)
				needBroadcast = true
				evs = append(evs, Event{Type: "plan-set", Key: def.Key, Name: def.Name, Steps: len(steps)})
			}
		}
	case "abandon":
		if e.canRunLocked("abandon") {
			e.abandonLocked(&evs)
			needBroadcast = true
		}
	case "resetToday":
		if e.canRunLocked("resetToday") {
			e.stats = store.TimerStats{Day: e.todayKey(), Rounds: 0, Minutes: 0}
			e.persistStatsLocked()
			e.st.round = 1
			e.persistLocked(true)
			needBroadcast = true
			if e.log != nil {
				e.log.Info("已结束今天，今日统计清零")
			}
		}
	case "refresh":
		if e.canRunLocked("refresh") {
			var snapOut Snapshot
			var bcb func(Snapshot)
			needBroadcast, snapOut, bcb = e.tickLocked(&evs, &notes)
			_ = snapOut
			_ = bcb
		}
	case "unbindTask":
		e.st.task = nil
		e.st.awaitPlan = false
		e.persistLocked(true)
		needBroadcast = true
	default:
		e.rejectLocked(cmd, "未知计时命令")
	}

	var snap Snapshot
	var bcb func(Snapshot)
	if needBroadcast {
		snap, bcb = e.prepareBroadcastLocked()
	} else {
		snap = e.snapshotLocked()
	}
	evCb := e.onEvent
	ntCb := e.notify
	e.mu.Unlock()

	if needBroadcast && bcb != nil {
		bcb(snap)
	}
	for _, ev := range evs {
		if evCb != nil {
			evCb(ev)
		}
	}
	for _, n := range notes {
		if ntCb != nil {
			ntCb(n[0], n[1])
		}
	}
	return snap
}

func (e *Engine) Tick() {
	e.mu.Lock()
	var evs []Event
	var notes [][2]string
	changed, snap, bcb := e.tickLocked(&evs, &notes)
	evCb := e.onEvent
	ntCb := e.notify
	e.mu.Unlock()

	if changed && bcb != nil {
		bcb(snap)
	}
	for _, ev := range evs {
		if evCb != nil {
			evCb(ev)
		}
	}
	for _, n := range notes {
		if ntCb != nil {
			ntCb(n[0], n[1])
		}
	}
}

func (e *Engine) tickLocked(evs *[]Event, notes *[][2]string) (bool, Snapshot, func(Snapshot)) {
	t := e.st
	wall := e.nowMs()
	m := e.monoMs()
	drift := (wall - e.lastWallMs) - (m - e.lastMonoMs)
	jumped := false
	if drift > ClockJumpLimit.Milliseconds() || drift < -ClockJumpLimit.Milliseconds() {
		e.clockJumpAt = wall
		jumped = true
		if e.log != nil {
			e.log.Warn(fmt.Sprintf("检测到系统时间跳变 %dms", drift))
		}
		if t.running {
			t.leftMs = e.currentLeftMsLocked()
			t.startMonoMs = m
			t.leftAtStartMs = t.leftMs
			t.endAtMs = wall + t.leftMs
		}
	}
	e.lastWallMs = wall
	e.lastMonoMs = m

	rolled := e.rollDayIfNeededLocked(false, evs)
	if !t.running {
		if jumped || rolled {
			snap, cb := e.prepareBroadcastLocked()
			return true, snap, cb
		}
		return false, Snapshot{}, nil
	}

	prevSec := int((t.leftMs + 999) / 1000)
	left := e.currentLeftMsLocked()
	t.leftMs = left
	t.endAtMs = wall + left
	nowSec := int((left + 999) / 1000)
	if left <= 0 {
		e.completeLocked(false, evs, notes)
		snap, cb := e.prepareBroadcastLocked()
		return true, snap, cb
	}
	if nowSec != prevSec || jumped || rolled {
		e.persistLocked(false)
		snap, cb := e.prepareBroadcastLocked()
		return true, snap, cb
	}
	return false, Snapshot{}, nil
}

// StartTask atomically validates and binds a today unfinished task to focus timer.
func (e *Engine) StartTask(id, day string, noStart bool) StartTaskResult {
	today := e.todayKey()
	if id == "" || len(id) > 128 || day != today {
		return StartTaskResult{Error: "请选择今天的有效任务。"}
	}
	var fs store.FocusState
	if !e.store.GetJSON(store.KeyStateV2, &fs) || fs.Days == nil {
		return StartTaskResult{Error: "任务不存在或已完成，请刷新任务列表。"}
	}
	bucket := fs.Days[day]
	var target *store.Task
	if bucket != nil {
		for i := range bucket.Tasks {
			if bucket.Tasks[i].ID == id {
				target = &bucket.Tasks[i]
				break
			}
		}
	}
	if target == nil || target.Done {
		return StartTaskResult{Error: "任务不存在或已完成，请刷新任务列表。"}
	}

	e.mu.Lock()
	var evs []Event
	if e.st.startedAt != "" {
		same := e.st.phase == "focus" && e.st.task != nil && e.st.task.ID == id && e.st.task.Day == day
		if !same {
			snap := e.snapshotLocked()
			e.mu.Unlock()
			return StartTaskResult{Code: "timer-busy", Error: "当前轮次尚未结束。请在专注钟中完成或重置本轮，再切换任务。", Snapshot: snap}
		}
		if !noStart && !e.st.running {
			e.setRunningLocked(true)
			snap, cb := e.prepareBroadcastLocked()
			e.mu.Unlock()
			if cb != nil {
				cb(snap)
			}
			return StartTaskResult{OK: true, Snapshot: snap}
		}
		snap := e.snapshotLocked()
		e.mu.Unlock()
		return StartTaskResult{OK: true, Snapshot: snap}
	}

	if e.st.phase != "focus" {
		e.st = e.newTimerLocked("focus")
	}
	e.st.task = &store.TaskRef{ID: target.ID, Day: day, Title: target.Title}
	if noStart {
		e.st.awaitPlan = true
		e.persistLocked(true)
		evs = append(evs, Event{Type: "await-plan", TaskID: target.ID, TaskTitle: target.Title})
	} else {
		e.setRunningLocked(true)
	}
	snap, cb := e.prepareBroadcastLocked()
	evCb := e.onEvent
	e.mu.Unlock()

	if cb != nil {
		cb(snap)
	}
	for _, ev := range evs {
		if evCb != nil {
			evCb(ev)
		}
	}
	return StartTaskResult{OK: true, Snapshot: snap}
}

func (e *Engine) ClearAwaitPlan() {
	e.mu.Lock()
	if e.st != nil && e.st.awaitPlan {
		e.st.awaitPlan = false
		snap, cb := e.prepareBroadcastLocked()
		e.mu.Unlock()
		if cb != nil {
			cb(snap)
		}
		return
	}
	e.mu.Unlock()
}

func (e *Engine) ResumeAfterWake() {
	e.StartTicker()
	e.Tick()
}

func (e *Engine) PauseBeforeSleep() {
	e.StopTicker()
}

// MarshalSnapshot is useful for diagnostics or serialization checks.
func (s Snapshot) JSON() string {
	b, _ := json.Marshal(s)
	return string(b)
}
