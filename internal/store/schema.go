package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
)

const (
	SchemaVersion  = 2
	KeyPrefix      = "sisy-"
	MaxKeyLen      = 128
	MaxValueBytes  = 5 * 1024 * 1024
	MaxBundleBytes = 32 * 1024 * 1024
	HistoryMax     = 300
	MaxTemplateLen = 32 * 1024
)

const (
	KeyStateV2           = "sisy-focus-state-v2"
	KeyStateV1           = "sisy-focus-state"
	KeyDaily             = "sisy-daily-config"
	KeyTimerState        = "sisy-timer-state"
	KeyTimerRun          = "sisy-timer-run"
	KeyTimerStats        = "sisy-timer-stats"
	KeyTimerPrefs        = "sisy-timer-prefs"
	KeyTimerHistory      = "sisy-timer-history"
	KeyTodoView          = "sisy-todo-view"
	KeyTimelineOverrides = "sisy-timeline-overrides"
	KeyExportTemplate    = "sisy-export-template"
	KeySeaScene          = "sisy-sea-scene"
)

var (
	keyRe      = regexp.MustCompile(`^sisy-[A-Za-z0-9][A-Za-z0-9._-]*$`)
	PhaseSet   = []string{"focus", "short", "long"}
	ThemeSet   = []string{"light", "dark", "system"}
	MotionSet  = []string{"full", "calm", "off"}
	WeatherSet = []string{"clear", "rain", "snow", "storm", "fog", "live"}
	PlanKeys   = []string{"classic", "deep", "sprint", "marathon"}
	ResultSet  = []string{"done", "abandoned", "recovered"}
	KnownKeys  = []string{KeyStateV2, KeyStateV1, KeyDaily, KeyTimerState, KeyTimerRun, KeyTimerStats, KeyTimerPrefs, KeyTimerHistory, KeyTodoView, KeyTimelineOverrides, KeyExportTemplate, KeySeaScene}
	ClearScope = map[string][]string{
		"tasks":      {KeyStateV2, KeyStateV1, KeyTimelineOverrides},
		"daily":      {KeyDaily},
		"stats":      {KeyTimerStats},
		"history":    {KeyTimerHistory},
		"prefs":      {KeyTimerPrefs, KeyTodoView, KeyExportTemplate, KeySeaScene},
		"timerState": {KeyTimerState, KeyTimerRun},
	}
)

// Envelope represents the on-disk sisy-store.json root object.
type Envelope struct {
	Schema    int                        `json:"schema"`
	UpdatedAt string                     `json:"updatedAt"`
	Data      map[string]json.RawMessage `json:"data"`
}

// ExportBundle represents exported backup files.
type ExportBundle struct {
	App        string                     `json:"app"`
	Schema     int                        `json:"schema"`
	ExportedAt string                     `json:"exportedAt"`
	Backend    string                     `json:"backend"`
	Data       map[string]json.RawMessage `json:"data"`
}

// Subtask matches JS schema for a task step.
type Subtask struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Minutes int    `json:"minutes"`
	Done    bool   `json:"done"`
}

// Task matches JS schema for a daily todo item.
type Task struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Done       bool      `json:"done"`
	Subtasks   []Subtask `json:"subtasks"`
	DailyID    string    `json:"dailyId,omitempty"`
	CreatedAt  string    `json:"createdAt,omitempty"`
	Alarm      string    `json:"alarm,omitempty"`
	RolledFrom string    `json:"rolledFrom,omitempty"`
}

// DayBucket holds one day's tasks and dismissed daily template IDs.
type DayBucket struct {
	Tasks          []Task          `json:"tasks"`
	DismissedDaily map[string]bool `json:"dismissedDaily"`
}

// FocusState corresponds to sisy-focus-state-v2.
type FocusState struct {
	ViewDay string                `json:"viewDay"`
	Days    map[string]*DayBucket `json:"days"`
}

// DailyTemplate is an entry in sisy-daily-config.
type DailyTemplate struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// DailyConfig corresponds to sisy-daily-config.
type DailyConfig struct {
	Tasks []DailyTemplate `json:"tasks"`
}

// TaskRef is a bound task snapshot inside timer state.
type TaskRef struct {
	ID    string `json:"id"`
	Day   string `json:"day"`
	Title string `json:"title"`
}

// PlanStep represents a single segment inside a multi-round pomodoro plan.
type PlanStep struct {
	Phase string `json:"phase"`
	Sec   int    `json:"sec"`
}

// PlanState holds an expanded pomodoro plan queue.
type PlanState struct {
	Key   string     `json:"key"`
	Name  string     `json:"name"`
	Steps []PlanStep `json:"steps"`
	Index int        `json:"index"`
}

// PersistedTimerState corresponds to sisy-timer-state.
type PersistedTimerState struct {
	Phase          string     `json:"phase"`
	TotalSec       int        `json:"totalSec"`
	LeftMs         int64      `json:"leftMs"`
	Running        bool       `json:"running"`
	EndAt          int64      `json:"endAt"`
	StartedAt      *string    `json:"startedAt"`
	PauseCount     int        `json:"pauseCount"`
	PausedMs       int64      `json:"pausedMs"`
	PauseStartedAt int64      `json:"pauseStartedAt"`
	Round          int        `json:"round"`
	Abandoned      bool       `json:"abandoned"`
	UpdatedAt      string     `json:"updatedAt,omitempty"`
	Task           *TaskRef   `json:"task"`
	Plan           *PlanState `json:"plan"`
	AwaitPlan      bool       `json:"awaitPlan,omitempty"`
}

// TimerStats corresponds to sisy-timer-stats.
type TimerStats struct {
	Day       string `json:"day"`
	Rounds    int    `json:"rounds"`
	Minutes   int    `json:"minutes"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// PhaseDurations stores phase seconds in prefs.
type PhaseDurations struct {
	Focus int `json:"focus,omitempty"`
	Short int `json:"short,omitempty"`
	Long  int `json:"long,omitempty"`
}

// TimerPrefs corresponds to sisy-timer-prefs.
type TimerPrefs struct {
	Sound        *bool           `json:"sound,omitempty"`
	Notify       *bool           `json:"notify,omitempty"`
	Motion       string          `json:"motion,omitempty"`
	Theme        string          `json:"theme,omitempty"`
	Tray         *bool           `json:"tray,omitempty"`
	CloseToTray  *bool           `json:"closeToTray,omitempty"`
	HighContrast *bool           `json:"highContrast,omitempty"`
	ShowSeconds  *bool           `json:"showSeconds,omitempty"`
	Phases       *PhaseDurations `json:"phases,omitempty"`
}

// HistoryEntry corresponds to one item in sisy-timer-history.
type HistoryEntry struct {
	Day            string `json:"day"`
	Phase          string `json:"phase"`
	StartedAt      string `json:"startedAt,omitempty"`
	FinishedAt     string `json:"finishedAt,omitempty"`
	PlannedMinutes int    `json:"plannedMinutes"`
	ActualMinutes  int    `json:"actualMinutes"`
	PauseCount     int    `json:"pauseCount"`
	PausedMinutes  int    `json:"pausedMinutes"`
	Result         string `json:"result"`
	TaskID         string `json:"taskId,omitempty"`
	TaskDay        string `json:"taskDay,omitempty"`
	TaskTitle      string `json:"taskTitle,omitempty"`
}

// TodoViewPrefs corresponds to sisy-todo-view.
type TodoViewPrefs struct {
	OnlyOpen  bool     `json:"onlyOpen"`
	Collapsed []string `json:"collapsed"`
	// Window layout the user left behind; optional so older stores stay valid.
	SidebarCollapsed bool `json:"sidebarCollapsed,omitempty"`
	TimelineOpen     bool `json:"timelineOpen,omitempty"`
	DoneGroupOpen    bool `json:"doneGroupOpen,omitempty"`
}

// TimelineEdit overrides a derived timeline block.
type TimelineEdit struct {
	Key    string `json:"key"`
	Start  string `json:"start,omitempty"`
	End    string `json:"end,omitempty"`
	Hidden *bool  `json:"hidden,omitempty"`
}

// ManualBlock is a user-added timeline block.
type ManualBlock struct {
	ID    string `json:"id"`
	Day   string `json:"day"`
	Start string `json:"start"`
	End   string `json:"end"`
	Title string `json:"title"`
}

// TimelineOverrides corresponds to sisy-timeline-overrides.
type TimelineOverrides struct {
	Edits  []TimelineEdit `json:"edits"`
	Manual []ManualBlock  `json:"manual"`
}

// NoteTemplateEntry holds a single markdown template.
type NoteTemplateEntry struct {
	Name string `json:"name"`
	Text string `json:"text"`
}

// ExportTemplate corresponds to sisy-export-template.
type ExportTemplate struct {
	Templates map[string]NoteTemplateEntry `json:"templates"`
}

// FlowPreset captures a 1-click focus atmosphere bundle (theme + weather + duration + ambient noise).
type FlowPreset struct {
	Name          string  `json:"name"`
	Theme         string  `json:"theme"`         // light | dark | system
	WeatherMode   string  `json:"weatherMode"`   // clear | rain | snow | storm | fog | live
	FocusMin      int     `json:"focusMin"`      // 1..180
	AmbientOn     bool    `json:"ambientOn"`
	AmbientVolume float64 `json:"ambientVolume"` // 0..1
}

// SeaSceneConfig corresponds to sisy-sea-scene (weather, time-of-day lighting, ambient sound, presets, encounters & atlas).
type SeaSceneConfig struct {
	WeatherMode   string            `json:"weatherMode"`   // clear | rain | snow | storm | fog | live
	TimeLighting  bool              `json:"timeLighting"`  // follow local time-of-day sky lighting
	EncountersOn  bool              `json:"encountersOn"`  // random ocean easter eggs
	AmbientOn     bool              `json:"ambientOn,omitempty"`
	AmbientVolume float64           `json:"ambientVolume,omitempty"` // 0..1 (default 0.45)
	City          string            `json:"city,omitempty"`
	ResolvedCity  string            `json:"resolvedCity,omitempty"`
	Lat           float64           `json:"lat,omitempty"`
	Lon           float64           `json:"lon,omitempty"`
	LiveWeather   string            `json:"liveWeather,omitempty"` // clear | rain | snow | storm | fog
	LiveTempC     *float64          `json:"liveTempC,omitempty"`
	LiveUpdatedAt string            `json:"liveUpdatedAt,omitempty"`
	Sightings     map[string]int    `json:"sightings,omitempty"` // {"angler":n, "pirate":n, "whale":n, ...}
	LastSeen      map[string]string `json:"lastSeen,omitempty"`  // {"whale":"YYYY-MM-DD"}
	FishCaught    int               `json:"fishCaught,omitempty"`
	CustomPreset  *FlowPreset       `json:"customPreset,omitempty"`
}

func DefaultSeaSceneConfig() SeaSceneConfig {
	return SeaSceneConfig{
		WeatherMode:   "clear",
		TimeLighting:  true,
		EncountersOn:  true,
		AmbientOn:     false,
		AmbientVolume: 0.45,
		LiveWeather:   "clear",
		Sightings:     map[string]int{},
		LastSeen:      map[string]string{},
	}
}

func (c *SeaSceneConfig) Normalize() {
	if !slices.Contains(WeatherSet, c.WeatherMode) {
		c.WeatherMode = "clear"
	}
	if !slices.Contains([]string{"clear", "rain", "snow", "storm", "fog"}, c.LiveWeather) {
		c.LiveWeather = "clear"
	}
	if c.AmbientVolume <= 0 || c.AmbientVolume > 1 {
		if c.AmbientVolume <= 0 {
			c.AmbientVolume = 0.45
		} else {
			c.AmbientVolume = 1.0
		}
	}
	if c.FishCaught < 0 {
		c.FishCaught = 0
	}
	if len([]rune(c.City)) > 64 {
		c.City = string([]rune(c.City)[:64])
	}
	if len([]rune(c.ResolvedCity)) > 96 {
		c.ResolvedCity = string([]rune(c.ResolvedCity)[:96])
	}
	if c.Sightings == nil {
		c.Sightings = map[string]int{}
	}
	if c.LastSeen == nil {
		c.LastSeen = map[string]string{}
	}
	if c.CustomPreset != nil {
		p := c.CustomPreset
		if strings.TrimSpace(p.Name) == "" {
			p.Name = "我的预设"
		}
		if len([]rune(p.Name)) > 24 {
			p.Name = string([]rune(p.Name)[:24])
		}
		if !slices.Contains(ThemeSet, p.Theme) {
			p.Theme = "light"
		}
		if !slices.Contains(WeatherSet, p.WeatherMode) {
			p.WeatherMode = "clear"
		}
		p.FocusMin = ClampInt(p.FocusMin, 1, 180)
		if p.AmbientVolume <= 0 || p.AmbientVolume > 1 {
			p.AmbientVolume = 0.45
		}
	}
}

// ImportSummary describes backup contents without leaking task titles.
type ImportSummary struct {
	ExportedAt      string `json:"exportedAt,omitempty"`
	App             string `json:"app,omitempty"`
	Schema          int    `json:"schema,omitempty"`
	Keys            int    `json:"keys"`
	Days            int    `json:"days"`
	Tasks           int    `json:"tasks"`
	DailyTemplates  int    `json:"dailyTemplates"`
	TimerMinutes    int    `json:"timerMinutes"`
	TimerRounds     int    `json:"timerRounds"`
	HistoryEntries  int    `json:"historyEntries"`
	RecoveryEntries int    `json:"recoveryEntries"`
}

type ValidationResult struct {
	OK       bool
	Errors   []string
	Repaired []string
	Raw      json.RawMessage
}

type BundleValidation struct {
	OK       bool
	Errors   []string
	Repaired []string
	PerKey   map[string]json.RawMessage
	Corrupt  map[string]json.RawMessage
	Summary  *ImportSummary
}

type ClearScopeResult struct {
	All  bool
	Keys []string
	Bad  []string
}

func IsValidStoreKey(k string) bool {
	return len(k) > len(KeyPrefix) && len(k) <= MaxKeyLen && strings.HasPrefix(k, KeyPrefix) && keyRe.MatchString(k)
}

func IsCorruptKey(k string) bool {
	return strings.Contains(k, "__corrupt_")
}

func IsKnownKey(k string) bool {
	return slices.Contains(KnownKeys, k)
}

func RandID(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s%x%s", prefix, time.Now().UnixNano(), hex.EncodeToString(b[:]))
}

func ClampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func ResolveClearScopes(scopes []string) ClearScopeResult {
	if scopes == nil {
		return ClearScopeResult{All: true}
	}
	if len(scopes) == 0 {
		return ClearScopeResult{Bad: []string{"空列表"}}
	}
	var out []string
	var bad []string
	seen := map[string]bool{}
	add := func(k string) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for _, s := range scopes {
		if s == "all" {
			for _, ks := range ClearScope {
				for _, k := range ks {
					add(k)
				}
			}
			continue
		}
		if ks, ok := ClearScope[s]; ok {
			for _, k := range ks {
				add(k)
			}
			continue
		}
		if IsValidStoreKey(s) && !IsCorruptKey(s) {
			add(s)
			continue
		}
		bad = append(bad, s)
	}
	return ClearScopeResult{All: false, Keys: out, Bad: bad}
}

func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if len(raw) == 0 || raw[0] != '{' {
		trimmed := strings.TrimSpace(string(raw))
		if !strings.HasPrefix(trimmed, "{") {
			return nil, false
		}
	}
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, false
	}
	return m, true
}

func decodeArray(raw json.RawMessage) ([]json.RawMessage, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "[") {
		return nil, false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil || arr == nil {
		return nil, false
	}
	return arr, true
}

func asStringLoose(raw json.RawMessage) (string, bool, bool) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", true, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, false, true
	}
	var v any
	if err := json.Unmarshal(raw, &v); err == nil {
		return fmt.Sprint(v), false, false
	}
	return trimmed, false, false
}

func asBool(raw json.RawMessage) (bool, bool) {
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, true
	}
	return false, false
}

func asFloat(raw json.RawMessage) (float64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f, true
	}
	return 0, false
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return json.RawMessage(b)
}

// ValidateValue validates a single key's JSON payload against schema rules.
func ValidateValue(key string, raw json.RawMessage) ValidationResult {
	res := ValidationResult{OK: true, Raw: raw}
	if len(raw) == 0 {
		res.OK = false
		res.Errors = append(res.Errors, key+"：值为空")
		return res
	}
	if IsCorruptKey(key) {
		m, ok := decodeObject(raw)
		if !ok {
			res.OK = false
			res.Errors = append(res.Errors, key+"：恢复键内容异常")
			return res
		}
		var r string
		if rawStr, exists := m["raw"]; !exists || json.Unmarshal(rawStr, &r) != nil {
			res.OK = false
			res.Errors = append(res.Errors, key+"：恢复键内容异常")
		}
		return res
	}

	var errs, repaired []string
	switch key {
	case KeyTodoView:
		m, ok := decodeObject(raw)
		if !ok {
			errs = append(errs, "任务视图偏好格式非法")
			break
		}
		ooRaw, hasOO := m["onlyOpen"]
		oo, ooOK := asBool(ooRaw)
		colRaw, hasCol := m["collapsed"]
		var col []string
		colErr := !hasCol || json.Unmarshal(colRaw, &col) != nil || col == nil || len(col) > 1000
		if !colErr {
			for _, item := range col {
				if len(item) > 256 {
					colErr = true
					break
				}
			}
		}
		if !hasOO || !ooOK || colErr {
			errs = append(errs, "任务视图偏好格式非法")
			break
		}
		optBool := func(name string) bool {
			v, _ := asBool(m[name])
			return v
		}
		res.Raw = mustMarshal(TodoViewPrefs{
			OnlyOpen:         oo,
			Collapsed:        col,
			SidebarCollapsed: optBool("sidebarCollapsed"),
			TimelineOpen:     optBool("timelineOpen"),
			DoneGroupOpen:    optBool("doneGroupOpen"),
		})

	case KeyStateV2, KeyStateV1:
		st := validateFocusState(raw, key, &errs, &repaired)
		if len(errs) == 0 && st != nil {
			res.Raw = mustMarshal(st)
		}

	case KeyDaily:
		dc := validateDailyConfig(raw, &errs)
		if len(errs) == 0 && dc != nil {
			res.Raw = mustMarshal(dc)
		}

	case KeyTimerStats:
		st := validateTimerStats(raw, &errs, &repaired)
		if len(errs) == 0 && st != nil {
			res.Raw = mustMarshal(st)
		}

	case KeyTimerPrefs:
		pr := validateTimerPrefs(raw, &errs, &repaired)
		if len(errs) == 0 && pr != nil {
			res.Raw = mustMarshal(pr)
		}

	case KeySeaScene:
		if _, ok := decodeObject(raw); !ok {
			errs = append(errs, "海域场景配置格式非法")
			break
		}
		sc := DefaultSeaSceneConfig()
		if err := json.Unmarshal(raw, &sc); err != nil {
			errs = append(errs, "海域场景配置解析失败")
			break
		}
		sc.Normalize()
		res.Raw = mustMarshal(sc)

	case KeyTimerState:
		ts := validateTimerState(raw, &errs, &repaired)
		if len(errs) == 0 && ts != nil {
			res.Raw = mustMarshal(ts)
		}

	case KeyTimerHistory:
		hist := validateTimerHistory(raw, &errs, &repaired)
		if len(errs) == 0 {
			res.Raw = mustMarshal(hist)
		}

	case KeyTimelineOverrides:
		ov := validateTimelineOverrides(raw, &errs, &repaired)
		if len(errs) == 0 && ov != nil {
			res.Raw = mustMarshal(ov)
		}

	case KeyExportTemplate:
		tpl := validateExportTemplate(raw, &errs, &repaired)
		if len(errs) == 0 && tpl != nil {
			res.Raw = mustMarshal(tpl)
		}

	case KeyTimerRun:
		if _, ok := decodeObject(raw); !ok {
			errs = append(errs, KeyTimerRun+"：必须是对象")
		}
	}

	res.OK = len(errs) == 0
	res.Errors = errs
	res.Repaired = repaired
	return res
}

func checkTask(raw json.RawMessage, label string, errs, repaired *[]string) (Task, bool) {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, label+"：任务必须是对象")
		return Task{}, false
	}
	var out Task
	if idRaw, ok := m["id"]; ok {
		var id string
		if json.Unmarshal(idRaw, &id) == nil && id != "" {
			out.ID = id
		}
	}
	if out.ID == "" {
		out.ID = RandID("t")
		*repaired = append(*repaired, label+"：任务缺 id，已补")
	}

	if tRaw, ok := m["title"]; ok {
		s, isNull, wasStr := asStringLoose(tRaw)
		if isNull {
			out.Title = ""
			*repaired = append(*repaired, label+"：任务缺 title，按空处理")
		} else {
			out.Title = s
			if !wasStr {
				*repaired = append(*repaired, label+"：任务 title 非字符串，已转")
			}
		}
	} else {
		*repaired = append(*repaired, label+"：任务缺 title，按空处理")
	}

	if dRaw, ok := m["done"]; ok {
		b, _ := asBool(dRaw)
		out.Done = b
	}

	out.Subtasks = []Subtask{}
	if subRaw, ok := m["subtasks"]; ok && strings.TrimSpace(string(subRaw)) != "null" {
		arr, ok := decodeArray(subRaw)
		if !ok {
			*repaired = append(*repaired, label+"：subtasks 非数组，已置空")
		} else {
			for i, sr := range arr {
				sl := fmt.Sprintf("%s.subtasks[%d]", label, i)
				sm, sok := decodeObject(sr)
				if !sok {
					*errs = append(*errs, sl+"：小步骤必须是对象")
					continue
				}
				var sub Subtask
				if sidRaw, ok := sm["id"]; ok {
					var sid string
					if json.Unmarshal(sidRaw, &sid) == nil && sid != "" {
						sub.ID = sid
					}
				}
				if sub.ID == "" {
					sub.ID = RandID("s")
					*repaired = append(*repaired, sl+"：缺 id，已补")
				}
				if stRaw, ok := sm["title"]; ok {
					s, isNull, wasStr := asStringLoose(stRaw)
					if isNull {
						sub.Title = ""
						*repaired = append(*repaired, sl+"：title 非字符串，已转")
					} else {
						sub.Title = s
						if !wasStr {
							*repaired = append(*repaired, sl+"：title 非字符串，已转")
						}
					}
				} else {
					*repaired = append(*repaired, sl+"：title 非字符串，已转")
				}
				sub.Minutes = 5
				if minRaw, ok := sm["minutes"]; ok {
					if f, fok := asFloat(minRaw); fok {
						sub.Minutes = ClampInt(int(f+0.5), 1, 180)
					}
				}
				if doneRaw, ok := sm["done"]; ok {
					sub.Done, _ = asBool(doneRaw)
				}
				out.Subtasks = append(out.Subtasks, sub)
			}
		}
	}

	if didRaw, ok := m["dailyId"]; ok {
		var did string
		if json.Unmarshal(didRaw, &did) == nil && did != "" {
			out.DailyID = did
		}
	}
	if caRaw, ok := m["createdAt"]; ok {
		var ca string
		if json.Unmarshal(caRaw, &ca) == nil {
			out.CreatedAt = ca
		}
	}
	if alRaw, ok := m["alarm"]; ok && strings.TrimSpace(string(alRaw)) != "null" {
		var al string
		if json.Unmarshal(alRaw, &al) == nil && dateutil.IsValidAlarm(al) {
			out.Alarm = al
		} else {
			*repaired = append(*repaired, label+"：alarm 非 HH:MM，已丢弃")
		}
	}
	if rfRaw, ok := m["rolledFrom"]; ok && strings.TrimSpace(string(rfRaw)) != "null" {
		var rf string
		if json.Unmarshal(rfRaw, &rf) == nil {
			if nk := dateutil.NormalizeKey(rf); nk != "" {
				out.RolledFrom = nk
			} else {
				*repaired = append(*repaired, label+"：rolledFrom 非法，已丢弃")
			}
		} else {
			*repaired = append(*repaired, label+"：rolledFrom 非法，已丢弃")
		}
	}
	if out.Title == "" && len(out.Subtasks) == 0 {
		*errs = append(*errs, label+"：任务既无标题也无小步骤")
		return Task{}, false
	}
	return out, true
}

func validateFocusState(raw json.RawMessage, label string, errs, repaired *[]string) *FocusState {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, label+"：必须是对象")
		return nil
	}
	var daysMap map[string]json.RawMessage
	migrated := false
	if dRaw, hasDays := m["days"]; hasDays {
		if dm, dok := decodeObject(dRaw); dok {
			daysMap = dm
		}
	}
	if daysMap == nil {
		if tRaw, hasTasks := m["tasks"]; hasTasks {
			if _, aok := decodeArray(tRaw); aok {
				today := dateutil.TodayKey(time.Now())
				bucket := mustMarshal(map[string]any{"tasks": tRaw, "dismissedDaily": map[string]bool{}})
				daysMap = map[string]json.RawMessage{today: bucket}
				*repaired = append(*repaired, label+"：旧版扁平结构已迁移到 "+today)
				migrated = true
			}
		}
	}
	if daysMap == nil {
		*errs = append(*errs, label+"：缺少 days 对象（必需字段）")
		return nil
	}

	out := &FocusState{Days: map[string]*DayBucket{}}
	// Sort keys for deterministic merging and repair messages
	keys := make([]string, 0, len(daysMap))
	for k := range daysMap {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		nk := dateutil.NormalizeKey(k)
		if nk == "" {
			*errs = append(*errs, fmt.Sprintf("%s.days[%s]：日期键非法", label, k))
			continue
		}
		if nk != k {
			*repaired = append(*repaired, fmt.Sprintf("%s：日期键 %s 已补零为 %s", label, k, nk))
		}
		dl := fmt.Sprintf("%s.days[%s]", label, nk)
		ddRaw := daysMap[k]
		var ddObj map[string]json.RawMessage
		if strings.TrimSpace(string(ddRaw)) == "null" {
			ddObj = map[string]json.RawMessage{}
			*repaired = append(*repaired, dl+"：缺失，按空天处理")
		} else {
			var dok bool
			ddObj, dok = decodeObject(ddRaw)
			if !dok {
				*errs = append(*errs, dl+"：必须是对象")
				continue
			}
		}

		dayOut := &DayBucket{Tasks: []Task{}, DismissedDaily: map[string]bool{}}
		if tr, hasTasks := ddObj["tasks"]; !hasTasks || strings.TrimSpace(string(tr)) == "null" {
			*repaired = append(*repaired, dl+"：tasks 缺省，按空列表")
		} else if arr, aok := decodeArray(tr); !aok {
			*errs = append(*errs, dl+"：tasks 必须是数组")
		} else {
			seen := map[string]bool{}
			for i, item := range arr {
				fixed, ok := checkTask(item, fmt.Sprintf("%s.tasks[%d]", dl, i), errs, repaired)
				if !ok {
					continue
				}
				if seen[fixed.ID] {
					fixed.ID = RandID("t")
					*repaired = append(*repaired, dl+"：重复任务 id 已重编号")
				}
				seen[fixed.ID] = true
				dayOut.Tasks = append(dayOut.Tasks, fixed)
			}
		}

		if disRaw, hasDis := ddObj["dismissedDaily"]; !hasDis || strings.TrimSpace(string(disRaw)) == "null" {
			*repaired = append(*repaired, dl+"：dismissedDaily 缺省，按空处理")
		} else if disObj, dok := decodeObject(disRaw); !dok {
			*errs = append(*errs, dl+"：dismissedDaily 必须是对象")
		} else {
			for dk, dv := range disObj {
				if b, _ := asBool(dv); b {
					dayOut.DismissedDaily[dk] = true
				}
			}
		}

		if target, exists := out.Days[nk]; exists {
			seen := map[string]bool{}
			for _, t := range target.Tasks {
				seen[t.ID] = true
			}
			for _, t := range dayOut.Tasks {
				if !seen[t.ID] {
					target.Tasks = append(target.Tasks, t)
					seen[t.ID] = true
				}
			}
			for dk := range dayOut.DismissedDaily {
				target.DismissedDaily[dk] = true
			}
		} else {
			out.Days[nk] = dayOut
		}
	}

	if len(*errs) > 0 {
		return nil
	}

	rawVD := ""
	if vdRaw, ok := m["viewDay"]; ok {
		_ = json.Unmarshal(vdRaw, &rawVD)
	}
	viewDay := dateutil.NormalizeKey(rawVD)
	if viewDay == "" {
		sorted := make([]string, 0, len(out.Days))
		for k := range out.Days {
			sorted = append(sorted, k)
		}
		slices.Sort(sorted)
		if len(sorted) > 0 {
			viewDay = sorted[len(sorted)-1]
		} else {
			viewDay = dateutil.TodayKey(time.Now())
		}
		*repaired = append(*repaired, label+"：viewDay 非法，已定为 "+viewDay)
	} else if !migrated && rawVD != "" && viewDay != rawVD {
		*repaired = append(*repaired, fmt.Sprintf("%s：viewDay %s 已补零", label, rawVD))
	}
	if out.Days[viewDay] == nil {
		out.Days[viewDay] = &DayBucket{Tasks: []Task{}, DismissedDaily: map[string]bool{}}
	}
	out.ViewDay = viewDay
	return out
}

func validateDailyConfig(raw json.RawMessage, errs *[]string) *DailyConfig {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, KeyDaily+"：必须是对象")
		return nil
	}
	tr, hasTasks := m["tasks"]
	arr, aok := decodeArray(tr)
	if !hasTasks || !aok {
		*errs = append(*errs, KeyDaily+"：缺少 tasks 数组（必需字段）")
		return nil
	}
	out := &DailyConfig{Tasks: []DailyTemplate{}}
	for i, item := range arr {
		tl := fmt.Sprintf("%s.tasks[%d]", KeyDaily, i)
		tm, tok := decodeObject(item)
		if !tok {
			*errs = append(*errs, tl+"：必须是对象")
			continue
		}
		var title string
		if tRaw, ok := tm["title"]; ok {
			_ = json.Unmarshal(tRaw, &title)
		}
		title = strings.TrimSpace(title)
		if title == "" {
			*errs = append(*errs, tl+"：缺少 title（必需字段）")
			continue
		}
		var id string
		if idRaw, ok := tm["id"]; ok {
			_ = json.Unmarshal(idRaw, &id)
		}
		if id == "" {
			id = RandID("d")
		}
		out.Tasks = append(out.Tasks, DailyTemplate{ID: id, Title: title})
	}
	return out
}

func validateTimerStats(raw json.RawMessage, errs, repaired *[]string) *TimerStats {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, KeyTimerStats+"：必须是对象")
		return nil
	}
	var rawDay string
	if dRaw, ok := m["day"]; ok {
		_ = json.Unmarshal(dRaw, &rawDay)
	}
	day := dateutil.NormalizeKey(rawDay)
	if day == "" {
		*errs = append(*errs, KeyTimerStats+"：day 缺失或非法（必需字段）")
		return nil
	}
	if day != rawDay {
		*repaired = append(*repaired, fmt.Sprintf("%s：day %s 已补零为 %s", KeyTimerStats, rawDay, day))
	}
	rRaw, hasR := m["rounds"]
	roundsF, rOK := asFloat(rRaw)
	if !hasR || !rOK {
		*errs = append(*errs, KeyTimerStats+"：rounds 缺失或非数字（必需字段）")
		return nil
	}
	mRaw, hasM := m["minutes"]
	minsF, mOK := asFloat(mRaw)
	if !hasM || !mOK {
		*errs = append(*errs, KeyTimerStats+"：minutes 缺失或非数字（必需字段）")
		return nil
	}
	out := &TimerStats{
		Day:     day,
		Rounds:  max(0, int(roundsF+0.5)),
		Minutes: max(0, int(minsF+0.5)),
	}
	if uaRaw, ok := m["updatedAt"]; ok {
		var ua string
		if json.Unmarshal(uaRaw, &ua) == nil {
			out.UpdatedAt = ua
		}
	}
	return out
}

func validateTimerPrefs(raw json.RawMessage, errs, repaired *[]string) *TimerPrefs {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, KeyTimerPrefs+"：必须是对象")
		return nil
	}
	out := &TimerPrefs{}
	setBool := func(k string, dst **bool) {
		vRaw, exists := m[k]
		if !exists || strings.TrimSpace(string(vRaw)) == "null" {
			return
		}
		if b, bok := asBool(vRaw); bok {
			*dst = &b
		} else {
			*repaired = append(*repaired, KeyTimerPrefs+"："+k+" 非布尔，已丢弃")
		}
	}
	setBool("sound", &out.Sound)
	setBool("notify", &out.Notify)
	setBool("tray", &out.Tray)
	setBool("closeToTray", &out.CloseToTray)
	setBool("highContrast", &out.HighContrast)
	setBool("showSeconds", &out.ShowSeconds)

	if mRaw, exists := m["motion"]; exists && strings.TrimSpace(string(mRaw)) != "null" {
		var s string
		if json.Unmarshal(mRaw, &s) == nil && slices.Contains(MotionSet, s) {
			out.Motion = s
		} else {
			*repaired = append(*repaired, KeyTimerPrefs+"：motion 非法值，已丢弃")
		}
	}
	if tRaw, exists := m["theme"]; exists && strings.TrimSpace(string(tRaw)) != "null" {
		var s string
		if json.Unmarshal(tRaw, &s) == nil && slices.Contains(ThemeSet, s) {
			out.Theme = s
		} else {
			*repaired = append(*repaired, KeyTimerPrefs+"：theme 非法值，已丢弃")
		}
	}
	if pRaw, exists := m["phases"]; exists && strings.TrimSpace(string(pRaw)) != "null" {
		pm, pok := decodeObject(pRaw)
		if !pok {
			*errs = append(*errs, KeyTimerPrefs+"：phases 必须是对象")
			return nil
		}
		pd := &PhaseDurations{}
		bad := false
		for _, k := range PhaseSet {
			vRaw, ok := pm[k]
			if !ok || strings.TrimSpace(string(vRaw)) == "null" {
				continue
			}
			f, fok := asFloat(vRaw)
			if !fok {
				*errs = append(*errs, fmt.Sprintf("%s.phases.%s：非数字时长", KeyTimerPrefs, k))
				bad = true
				continue
			}
			n := ClampInt(int(f+0.5), 60, 180*60)
			if float64(n) != f {
				*repaired = append(*repaired, fmt.Sprintf("%s.phases.%s 已规范到 %d", KeyTimerPrefs, k, n))
			}
			switch k {
			case "focus":
				pd.Focus = n
			case "short":
				pd.Short = n
			case "long":
				pd.Long = n
			}
		}
		if bad {
			return nil
		}
		var extra []string
		for k := range pm {
			if !slices.Contains(PhaseSet, k) {
				extra = append(extra, k)
			}
		}
		if len(extra) > 0 {
			slices.Sort(extra)
			*repaired = append(*repaired, KeyTimerPrefs+"：忽略未知阶段 "+strings.Join(extra, ","))
		}
		out.Phases = pd
	}
	return out
}

func validateTimerState(raw json.RawMessage, errs, repaired *[]string) *PersistedTimerState {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, KeyTimerState+"：必须是对象")
		return nil
	}
	var phase string
	if pRaw, ok := m["phase"]; ok {
		_ = json.Unmarshal(pRaw, &phase)
	}
	if !slices.Contains(PhaseSet, phase) {
		*errs = append(*errs, KeyTimerState+"：phase 非法（需 focus/short/long）")
		return nil
	}
	totalSec := 0
	if tsRaw, ok := m["totalSec"]; ok {
		if f, fok := asFloat(tsRaw); fok {
			totalSec = ClampInt(int(f+0.5), 1, 180*60)
		}
	}
	if totalSec <= 0 {
		*errs = append(*errs, KeyTimerState+"：totalSec 缺失或非法（必需字段）")
		return nil
	}
	leftMs := int64(totalSec * 1000)
	if lmRaw, ok := m["leftMs"]; ok {
		if f, fok := asFloat(lmRaw); fok {
			v := int64(f + 0.5)
			if v < 0 {
				v = 0
			} else if v > int64(totalSec)*1000 {
				v = int64(totalSec) * 1000
			}
			leftMs = v
		}
	}
	running := false
	if rRaw, ok := m["running"]; ok {
		running, _ = asBool(rRaw)
	}
	var endAt int64
	if running {
		eaRaw, ok := m["endAt"]
		f, fok := asFloat(eaRaw)
		if !ok || !fok || f <= 0 {
			*errs = append(*errs, KeyTimerState+"：running=true 但缺少合法 endAt（必需字段）")
			return nil
		}
		endAt = int64(f + 0.5)
	}

	out := &PersistedTimerState{
		Phase:    phase,
		TotalSec: totalSec,
		LeftMs:   leftMs,
		Running:  running,
		EndAt:    endAt,
		Round:    1,
	}
	if saRaw, ok := m["startedAt"]; ok {
		var sa string
		if json.Unmarshal(saRaw, &sa) == nil {
			out.StartedAt = &sa
		}
	}
	if pcRaw, ok := m["pauseCount"]; ok {
		if f, fok := asFloat(pcRaw); fok {
			out.PauseCount = ClampInt(int(f+0.5), 0, 1_000_000)
		}
	}
	if pmRaw, ok := m["pausedMs"]; ok {
		if f, fok := asFloat(pmRaw); fok {
			v := int64(f + 0.5)
			if v > 0 {
				out.PausedMs = v
			}
		}
	}
	if psaRaw, ok := m["pauseStartedAt"]; ok {
		if f, fok := asFloat(psaRaw); fok && f > 0 {
			out.PauseStartedAt = int64(f + 0.5)
		}
	}
	if rdRaw, ok := m["round"]; ok {
		if f, fok := asFloat(rdRaw); fok {
			out.Round = ClampInt(int(f+0.5), 1, 1_000_000)
		}
	}
	if abRaw, ok := m["abandoned"]; ok {
		out.Abandoned, _ = asBool(abRaw)
	}
	if uaRaw, ok := m["updatedAt"]; ok {
		var ua string
		if json.Unmarshal(uaRaw, &ua) == nil {
			out.UpdatedAt = ua
		}
	}
	if tRaw, exists := m["task"]; exists && strings.TrimSpace(string(tRaw)) != "null" {
		tm, tok := decodeObject(tRaw)
		if !tok {
			*errs = append(*errs, KeyTimerState+"：task 关联格式非法")
			return nil
		}
		var id, day, title string
		var idOK, dayOK, titleOK bool
		if v, ok := tm["id"]; ok && json.Unmarshal(v, &id) == nil && id != "" && len(id) <= 128 {
			idOK = true
		}
		if v, ok := tm["day"]; ok && json.Unmarshal(v, &day) == nil {
			if nk := dateutil.NormalizeKey(day); nk != "" {
				day = nk
				dayOK = true
			}
		}
		if v, ok := tm["title"]; ok && json.Unmarshal(v, &title) == nil && len(title) <= 500 {
			titleOK = true
		}
		if !idOK || !dayOK || !titleOK {
			*errs = append(*errs, KeyTimerState+"：task 关联格式非法")
			return nil
		}
		out.Task = &TaskRef{ID: id, Day: day, Title: title}
	}
	if pRaw, exists := m["plan"]; exists && strings.TrimSpace(string(pRaw)) != "null" {
		out.Plan = checkPlan(pRaw, KeyTimerState, repaired)
	}
	if apRaw, ok := m["awaitPlan"]; ok {
		out.AwaitPlan, _ = asBool(apRaw)
	}
	return out
}

func checkPlan(raw json.RawMessage, label string, repaired *[]string) *PlanState {
	m, ok := decodeObject(raw)
	if !ok {
		*repaired = append(*repaired, label+"：plan 结构非法，已丢弃")
		return nil
	}
	var key, name string
	var stepsRaw []json.RawMessage
	keyOK := false
	if v, ok := m["key"]; ok && json.Unmarshal(v, &key) == nil && key != "" {
		keyOK = true
	}
	nameOK := false
	if v, ok := m["name"]; ok && json.Unmarshal(v, &name) == nil && len(name) <= 60 {
		nameOK = true
	}
	if v, ok := m["steps"]; ok {
		if arr, aok := decodeArray(v); aok && len(arr) > 0 && len(arr) <= 60 {
			stepsRaw = arr
		}
	}
	if !keyOK || !nameOK || stepsRaw == nil {
		*repaired = append(*repaired, label+"：plan 结构非法，已丢弃")
		return nil
	}
	steps := make([]PlanStep, 0, len(stepsRaw))
	for _, sr := range stepsRaw {
		sm, sok := decodeObject(sr)
		if !sok {
			*repaired = append(*repaired, label+"：plan.steps 含非法段，已丢弃")
			return nil
		}
		var ph string
		if v, ok := sm["phase"]; !ok || json.Unmarshal(v, &ph) != nil || !slices.Contains(PhaseSet, ph) {
			*repaired = append(*repaired, label+"：plan.steps 含非法段，已丢弃")
			return nil
		}
		secRaw, hasSec := sm["sec"]
		f, fok := asFloat(secRaw)
		if !hasSec || !fok {
			*repaired = append(*repaired, label+"：plan.steps 含非法段，已丢弃")
			return nil
		}
		steps = append(steps, PlanStep{Phase: ph, Sec: ClampInt(int(f+0.5), 60, 180*60)})
	}
	idx := 0
	if idxRaw, ok := m["index"]; ok {
		if f, fok := asFloat(idxRaw); fok {
			idx = ClampInt(int(f+0.5), 0, len(steps)-1)
		}
	}
	if len(key) > 40 {
		key = key[:40]
	}
	return &PlanState{Key: key, Name: name, Steps: steps, Index: idx}
}

func validateTimerHistory(raw json.RawMessage, errs, repaired *[]string) []HistoryEntry {
	arr, ok := decodeArray(raw)
	if !ok {
		*errs = append(*errs, KeyTimerHistory+"：必须是数组")
		return nil
	}
	out := make([]HistoryEntry, 0, len(arr))
	for i, item := range arr {
		hl := fmt.Sprintf("%s[%d]", KeyTimerHistory, i)
		hm, hok := decodeObject(item)
		if !hok {
			*errs = append(*errs, hl+"：历史条目必须是对象")
			continue
		}
		var rawDay string
		if v, ok := hm["day"]; ok {
			_ = json.Unmarshal(v, &rawDay)
		}
		day := dateutil.NormalizeKey(rawDay)
		if day == "" {
			*errs = append(*errs, hl+"：day 缺失或非法（必需字段）")
			continue
		}
		var phase string
		if v, ok := hm["phase"]; !ok || json.Unmarshal(v, &phase) != nil || !slices.Contains(PhaseSet, phase) {
			*errs = append(*errs, hl+"：phase 非法")
			continue
		}
		var result string
		if v, ok := hm["result"]; !ok || json.Unmarshal(v, &result) != nil || !slices.Contains(ResultSet, result) {
			*errs = append(*errs, hl+"：result 非法")
			continue
		}
		entry := HistoryEntry{Day: day, Phase: phase, Result: result}
		if v, ok := hm["startedAt"]; ok {
			_ = json.Unmarshal(v, &entry.StartedAt)
		}
		if v, ok := hm["finishedAt"]; ok {
			_ = json.Unmarshal(v, &entry.FinishedAt)
		}
		if v, ok := hm["plannedMinutes"]; ok {
			if f, fok := asFloat(v); fok {
				entry.PlannedMinutes = ClampInt(int(f+0.5), 0, 1_000_000)
			}
		}
		if v, ok := hm["actualMinutes"]; ok {
			if f, fok := asFloat(v); fok {
				entry.ActualMinutes = ClampInt(int(f+0.5), 0, 1_000_000)
			}
		}
		if v, ok := hm["pauseCount"]; ok {
			if f, fok := asFloat(v); fok {
				entry.PauseCount = ClampInt(int(f+0.5), 0, 1_000_000)
			}
		}
		if v, ok := hm["pausedMinutes"]; ok {
			if f, fok := asFloat(v); fok {
				entry.PausedMinutes = ClampInt(int(f+0.5), 0, 1_000_000)
			}
		}
		if v, ok := hm["taskId"]; ok {
			var tid string
			if json.Unmarshal(v, &tid) == nil && len(tid) <= 128 {
				entry.TaskID = tid
			}
		}
		if v, ok := hm["taskDay"]; ok {
			var td string
			if json.Unmarshal(v, &td) == nil {
				if nk := dateutil.NormalizeKey(td); nk != "" {
					entry.TaskDay = nk
				}
			}
		}
		if v, ok := hm["taskTitle"]; ok {
			var tt string
			if json.Unmarshal(v, &tt) == nil {
				if len([]rune(tt)) > 500 {
					tt = string([]rune(tt)[:500])
				}
				entry.TaskTitle = tt
			}
		}
		out = append(out, entry)
	}
	if len(out) > HistoryMax {
		*repaired = append(*repaired, fmt.Sprintf("%s：超出上限，仅保留最近 %d 条", KeyTimerHistory, HistoryMax))
		out = out[len(out)-HistoryMax:]
	}
	return out
}

func validateTimelineOverrides(raw json.RawMessage, errs, repaired *[]string) *TimelineOverrides {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, KeyTimelineOverrides+"：必须是对象")
		return nil
	}
	out := &TimelineOverrides{Edits: []TimelineEdit{}, Manual: []ManualBlock{}}
	if eRaw, ok := m["edits"]; ok {
		if arr, aok := decodeArray(eRaw); aok {
			for i, item := range arr {
				el := fmt.Sprintf("%s.edits[%d]", KeyTimelineOverrides, i)
				em, eok := decodeObject(item)
				if !eok {
					*repaired = append(*repaired, el+"：非法，已丢弃")
					continue
				}
				var k string
				if v, ok := em["key"]; !ok || json.Unmarshal(v, &k) != nil || k == "" || len(k) > 200 {
					*repaired = append(*repaired, el+"：非法，已丢弃")
					continue
				}
				ed := TimelineEdit{Key: k}
				if v, ok := em["start"]; ok && strings.TrimSpace(string(v)) != "null" {
					var s string
					if json.Unmarshal(v, &s) == nil && dateutil.IsValidAlarm(s) {
						ed.Start = s
					} else {
						*repaired = append(*repaired, el+"：start 非 HH:MM，已丢弃")
					}
				}
				if v, ok := em["end"]; ok && strings.TrimSpace(string(v)) != "null" {
					var s string
					if json.Unmarshal(v, &s) == nil && dateutil.IsValidAlarm(s) {
						ed.End = s
					} else {
						*repaired = append(*repaired, el+"：end 非 HH:MM，已丢弃")
					}
				}
				if v, ok := em["hidden"]; ok && strings.TrimSpace(string(v)) != "null" {
					b, _ := asBool(v)
					ed.Hidden = &b
				}
				if ed.Start == "" && ed.End == "" && ed.Hidden == nil {
					*repaired = append(*repaired, el+"：无有效修改，已丢弃")
					continue
				}
				out.Edits = append(out.Edits, ed)
			}
		}
	}
	if mRaw, ok := m["manual"]; ok {
		if arr, aok := decodeArray(mRaw); aok {
			for i, item := range arr {
				ml := fmt.Sprintf("%s.manual[%d]", KeyTimelineOverrides, i)
				mm, mok := decodeObject(item)
				if !mok {
					*repaired = append(*repaired, ml+"：非法，已丢弃")
					continue
				}
				var dayRaw, st, en string
				if v, ok := mm["day"]; ok {
					_ = json.Unmarshal(v, &dayRaw)
				}
				day := dateutil.NormalizeKey(dayRaw)
				stOK := false
				if v, ok := mm["start"]; ok && json.Unmarshal(v, &st) == nil && dateutil.IsValidAlarm(st) {
					stOK = true
				}
				enOK := false
				if v, ok := mm["end"]; ok && json.Unmarshal(v, &en) == nil && dateutil.IsValidAlarm(en) {
					enOK = true
				}
				if day == "" || !stOK || !enOK {
					*repaired = append(*repaired, ml+"：字段缺失/非法，已丢弃")
					continue
				}
				if en <= st {
					*repaired = append(*repaired, ml+"：结束不晚于开始，已丢弃")
					continue
				}
				var id, title string
				if v, ok := mm["id"]; ok {
					_ = json.Unmarshal(v, &id)
				}
				if id == "" {
					id = RandID("m")
				} else if len(id) > 64 {
					id = id[:64]
				}
				if v, ok := mm["title"]; ok {
					_ = json.Unmarshal(v, &title)
				}
				if len([]rune(title)) > 200 {
					title = string([]rune(title)[:200])
				}
				out.Manual = append(out.Manual, ManualBlock{ID: id, Day: day, Start: st, End: en, Title: title})
			}
		}
	}
	return out
}

func validateExportTemplate(raw json.RawMessage, errs, repaired *[]string) *ExportTemplate {
	m, ok := decodeObject(raw)
	if !ok {
		*errs = append(*errs, KeyExportTemplate+"：必须是对象")
		return nil
	}
	out := &ExportTemplate{Templates: map[string]NoteTemplateEntry{}}
	var src map[string]json.RawMessage
	if tRaw, ok := m["templates"]; ok {
		src, _ = decodeObject(tRaw)
	}
	for _, kind := range []string{"daily", "task", "weekly"} {
		tr, exists := src[kind]
		if !exists || strings.TrimSpace(string(tr)) == "null" {
			continue
		}
		tm, tok := decodeObject(tr)
		var text string
		if !tok || json.Unmarshal(tm["text"], &text) != nil {
			*repaired = append(*repaired, fmt.Sprintf("%s.%s：结构非法，已丢弃", KeyExportTemplate, kind))
			continue
		}
		if len(text) > MaxTemplateLen {
			*repaired = append(*repaired, fmt.Sprintf("%s.%s：超长，已截断", KeyExportTemplate, kind))
			text = text[:MaxTemplateLen]
		}
		name := kind
		if v, ok := tm["name"]; ok {
			var n string
			if json.Unmarshal(v, &n) == nil && n != "" {
				if len([]rune(n)) > 60 {
					n = string([]rune(n)[:60])
				}
				name = n
			}
		}
		out.Templates[kind] = NoteTemplateEntry{Name: name, Text: text}
	}
	return out
}

// ValidateBundle validates an entire backup bundle before import.
func ValidateBundle(raw json.RawMessage) BundleValidation {
	res := BundleValidation{
		PerKey:  map[string]json.RawMessage{},
		Corrupt: map[string]json.RawMessage{},
	}
	root, ok := decodeObject(raw)
	if !ok {
		res.Errors = append(res.Errors, "备份文件必须是一个 JSON 对象")
		return res
	}
	dataRaw, hasData := root["data"]
	dataObj, dok := decodeObject(dataRaw)
	if !hasData || !dok {
		res.Errors = append(res.Errors, "缺少 data 字段或格式不对（不是有效的备份文件）")
		return res
	}
	keys := make([]string, 0, len(dataObj))
	for k := range dataObj {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		if !IsValidStoreKey(k) {
			res.Errors = append(res.Errors, "非法键名："+fmt.Sprintf("%q", k))
			continue
		}
		if IsCorruptKey(k) {
			res.Corrupt[k] = dataObj[k]
			continue
		}
		vr := ValidateValue(k, dataObj[k])
		for _, r := range vr.Repaired {
			res.Repaired = append(res.Repaired, "导入修复："+r)
		}
		if !vr.OK {
			res.Errors = append(res.Errors, vr.Errors...)
			continue
		}
		res.PerKey[k] = vr.Raw
	}
	if len(res.PerKey) == 0 && len(res.Corrupt) == 0 && len(keys) == 0 {
		res.Errors = append(res.Errors, "备份文件里没有任何可导入的数据")
	}
	var meta ExportBundle
	_ = json.Unmarshal(raw, &meta)
	res.Summary = Summarize(res.PerKey, len(res.Corrupt), meta.ExportedAt, meta.App, meta.Schema)
	res.OK = len(res.Errors) == 0
	return res
}

// Summarize produces non-sensitive counts for UI and logs.
func Summarize(data map[string]json.RawMessage, recoveryCount int, exportedAt, appName string, schema int) *ImportSummary {
	s := &ImportSummary{
		ExportedAt:      exportedAt,
		App:             appName,
		Schema:          schema,
		Keys:            len(data) + recoveryCount,
		RecoveryEntries: recoveryCount,
	}
	var fs FocusState
	if r, ok := data[KeyStateV2]; ok && json.Unmarshal(r, &fs) == nil {
		s.Days = len(fs.Days)
		for _, b := range fs.Days {
			if b != nil {
				s.Tasks += len(b.Tasks)
			}
		}
	} else if r, ok := data[KeyStateV1]; ok && recoveryCount == 0 && json.Unmarshal(r, &fs) == nil {
		s.Days = len(fs.Days)
		for _, b := range fs.Days {
			if b != nil {
				s.Tasks += len(b.Tasks)
			}
		}
	}
	var dc DailyConfig
	if r, ok := data[KeyDaily]; ok && json.Unmarshal(r, &dc) == nil {
		s.DailyTemplates = len(dc.Tasks)
	}
	var st TimerStats
	if r, ok := data[KeyTimerStats]; ok && json.Unmarshal(r, &st) == nil {
		s.TimerMinutes = st.Minutes
		s.TimerRounds = st.Rounds
	}
	var hist []HistoryEntry
	if r, ok := data[KeyTimerHistory]; ok && json.Unmarshal(r, &hist) == nil {
		s.HistoryEntries = len(hist)
	}
	return s
}
