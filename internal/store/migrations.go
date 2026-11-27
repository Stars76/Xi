package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
)

// MigrateResult holds the normalized envelope data and migration notes.
type MigrateResult struct {
	Data     map[string]json.RawMessage
	Schema   int
	Upgraded bool
	Notes    []string
}

func MigrateEnvelope(raw json.RawMessage) MigrateResult {
	var notes []string
	m, ok := decodeObject(raw)
	if !ok {
		notes = append(notes, "外层结构非法，按空存储处理")
		return MigrateResult{Data: map[string]json.RawMessage{}, Schema: SchemaVersion, Upgraded: true, Notes: notes}
	}

	var data map[string]json.RawMessage
	from := 0
	if dRaw, hasData := m["data"]; hasData {
		if dm, dok := decodeObject(dRaw); dok {
			data = dm
			if sRaw, ok := m["schema"]; ok {
				if f, fok := asFloat(sRaw); fok {
					from = int(f)
				}
			}
			if from == 0 {
				from = 1
			}
		}
	}
	if data == nil {
		data = map[string]json.RawMessage{}
		for k, v := range m {
			if k == "schema" || k == "updatedAt" {
				continue
			}
			data[k] = v
		}
		from = 1
		notes = append(notes, "检测到裸键值表，已包装为 {schema,data} 外层")
	}
	if from < 1 || from > SchemaVersion {
		notes = append(notes, fmt.Sprintf("未知外层版本 %d，按 %d -> %d 尽力升级", from, from, SchemaVersion))
	}
	changes := MigrateData(data)
	notes = append(notes, changes...)
	return MigrateResult{
		Data:     data,
		Schema:   SchemaVersion,
		Upgraded: from != SchemaVersion || len(changes) > 0,
		Notes:    notes,
	}
}

// MigrateData runs idempotent M1..M6 migrations in-place on data.
func MigrateData(data map[string]json.RawMessage) []string {
	if data == nil {
		return nil
	}
	var changes []string
	migrateFocusState(data, &changes)
	migrateStats(data, &changes)
	migrateHistory(data, &changes)
	migratePrefs(data, &changes)
	return changes
}

func migrateFocusState(data map[string]json.RawMessage, changes *[]string) {
	stRaw, hasV2 := data[KeyStateV2]
	if !hasV2 || strings.TrimSpace(string(stRaw)) == "null" {
		if v1Raw, hasV1 := data[KeyStateV1]; hasV1 && strings.TrimSpace(string(v1Raw)) != "null" {
			stRaw = v1Raw
			data[KeyStateV2] = v1Raw
			delete(data, KeyStateV1)
			*changes = append(*changes, fmt.Sprintf("M3 旧键 %s -> %s", KeyStateV1, KeyStateV2))
		} else {
			delete(data, KeyStateV1)
			return
		}
	}
	m, ok := decodeObject(stRaw)
	if !ok {
		*changes = append(*changes, fmt.Sprintf("M0 %s 非对象，移除", KeyStateV2))
		delete(data, KeyStateV2)
		return
	}

	// M1: flat {tasks:[...]} -> {viewDay, days:{today:{tasks}}}
	if _, hasDays := m["days"]; !hasDays {
		if tRaw, hasTasks := m["tasks"]; hasTasks {
			if _, aok := decodeArray(tRaw); aok {
				today := dateutil.TodayKey(time.Now())
				daysMap := map[string]any{
					today: map[string]any{"tasks": tRaw, "dismissedDaily": map[string]bool{}},
				}
				m = map[string]json.RawMessage{
					"viewDay": mustMarshal(today),
					"days":    mustMarshal(daysMap),
				}
				*changes = append(*changes, fmt.Sprintf("M1 扁平任务结构迁移到按日期分组（%s）", today))
				data[KeyStateV2] = mustMarshal(m)
			}
		}
	}

	daysRaw, hasDays := m["days"]
	daysMap, dok := decodeObject(daysRaw)
	if !hasDays || !dok {
		return
	}

	// M2: zero-pad date keys + merge conflicts deterministically
	touched := false
	fixedDays := map[string]*DayBucket{}
	rawDayObjects := map[string]map[string]json.RawMessage{}

	keys := make([]string, 0, len(daysMap))
	for k := range daysMap {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		nk := dateutil.NormalizeKey(k)
		if nk == "" {
			*changes = append(*changes, fmt.Sprintf("M2 %s 丢弃非法日期键 %q", KeyStateV2, k))
			touched = true
			continue
		}
		if nk != k {
			touched = true
			*changes = append(*changes, fmt.Sprintf("M2 日期键 %s -> %s", k, nk))
		}
		incMap, iok := decodeObject(daysMap[k])
		if !iok {
			touched = true
			continue
		}
		if _, exists := rawDayObjects[nk]; !exists {
			rawDayObjects[nk] = incMap
			continue
		}
		// Merge conflict day buckets
		touched = true
		var errs, rep []string
		b1 := fixedDays[nk]
		if b1 == nil {
			b1 = parseBucketLoose(rawDayObjects[nk])
			fixedDays[nk] = b1
		}
		b2 := parseBucketLoose(incMap)
		seen := map[string]bool{}
		for _, t := range b1.Tasks {
			if t.ID != "" {
				seen[t.ID] = true
			}
		}
		for _, t := range b2.Tasks {
			if t.ID != "" && seen[t.ID] {
				continue
			}
			b1.Tasks = append(b1.Tasks, t)
			if t.ID != "" {
				seen[t.ID] = true
			}
		}
		for dk, dv := range b2.DismissedDaily {
			if dv && !b1.DismissedDaily[dk] {
				b1.DismissedDaily[dk] = true
			}
		}
		_ = errs
		_ = rep
	}

	if touched {
		outDays := map[string]any{}
		for nk, rawObj := range rawDayObjects {
			if b, has := fixedDays[nk]; has {
				outDays[nk] = b
			} else {
				outDays[nk] = rawObj
			}
		}
		m["days"] = mustMarshal(outDays)
	}

	var vd string
	if v, ok := m["viewDay"]; ok {
		_ = json.Unmarshal(v, &vd)
	}
	nvd := dateutil.NormalizeKey(vd)
	if nvd != "" && nvd != vd {
		*changes = append(*changes, fmt.Sprintf("M2 viewDay %s -> %s", vd, nvd))
		m["viewDay"] = mustMarshal(nvd)
		touched = true
	} else if nvd == "" {
		sorted := make([]string, 0, len(rawDayObjects))
		for k := range rawDayObjects {
			sorted = append(sorted, k)
		}
		slices.Sort(sorted)
		fallback := dateutil.TodayKey(time.Now())
		if len(sorted) > 0 {
			fallback = sorted[len(sorted)-1]
		}
		m["viewDay"] = mustMarshal(fallback)
		*changes = append(*changes, "M2 viewDay 非法，重置为 "+fallback)
		touched = true
	}

	if touched {
		data[KeyStateV2] = mustMarshal(m)
	}
}

func parseBucketLoose(m map[string]json.RawMessage) *DayBucket {
	b := &DayBucket{Tasks: []Task{}, DismissedDaily: map[string]bool{}}
	if tr, ok := m["tasks"]; ok {
		var ts []Task
		if json.Unmarshal(tr, &ts) == nil && ts != nil {
			b.Tasks = ts
		}
	}
	if dr, ok := m["dismissedDaily"]; ok {
		var dm map[string]bool
		if json.Unmarshal(dr, &dm) == nil && dm != nil {
			b.DismissedDaily = dm
		}
	}
	return b
}

func migrateStats(data map[string]json.RawMessage, changes *[]string) {
	raw, ok := data[KeyTimerStats]
	if !ok {
		return
	}
	m, mok := decodeObject(raw)
	if !mok {
		if strings.TrimSpace(string(raw)) != "null" {
			delete(data, KeyTimerStats)
			*changes = append(*changes, "M4 非法统计已移除")
		}
		return
	}
	before := bytes.Clone(raw)
	var day string
	if v, ok := m["day"]; ok {
		_ = json.Unmarshal(v, &day)
	}
	if nd := dateutil.NormalizeKey(day); nd != "" && nd != day {
		m["day"] = mustMarshal(nd)
		*changes = append(*changes, "M4 统计日期键补零 -> "+nd)
	}
	if v, ok := m["rounds"]; ok {
		if f, fok := asFloat(v); fok {
			r := ClampInt(int(f+0.5), 0, 1_000_000)
			if float64(r) != f {
				m["rounds"] = mustMarshal(r)
				*changes = append(*changes, fmt.Sprintf("M4 rounds 规范化为 %d", r))
			}
		}
	} else {
		m["rounds"] = mustMarshal(0)
		*changes = append(*changes, "M4 rounds 规范化为 0")
	}
	if v, ok := m["minutes"]; ok {
		if f, fok := asFloat(v); fok {
			mn := ClampInt(int(f+0.5), 0, 1_000_000)
			if float64(mn) != f {
				m["minutes"] = mustMarshal(mn)
				*changes = append(*changes, fmt.Sprintf("M4 minutes 规范化为 %d", mn))
			}
		}
	} else {
		m["minutes"] = mustMarshal(0)
		*changes = append(*changes, "M4 minutes 规范化为 0")
	}
	after := mustMarshal(m)
	if !bytes.Equal(before, after) {
		data[KeyTimerStats] = after
	}
}

func migrateHistory(data map[string]json.RawMessage, changes *[]string) {
	raw, ok := data[KeyTimerHistory]
	if !ok || strings.TrimSpace(string(raw)) == "null" {
		return
	}
	arr, aok := decodeArray(raw)
	if !aok {
		delete(data, KeyTimerHistory)
		*changes = append(*changes, "M5 非法历史已移除")
		return
	}
	if len(arr) > HistoryMax {
		trimmed := arr[len(arr)-HistoryMax:]
		data[KeyTimerHistory] = mustMarshal(trimmed)
		*changes = append(*changes, fmt.Sprintf("M5 历史裁剪到最近 %d 条", HistoryMax))
	}
}

func migratePrefs(data map[string]json.RawMessage, changes *[]string) {
	raw, ok := data[KeyTimerPrefs]
	if !ok {
		return
	}
	m, mok := decodeObject(raw)
	if !mok {
		if strings.TrimSpace(string(raw)) != "null" {
			delete(data, KeyTimerPrefs)
			*changes = append(*changes, "M6 非法偏好已移除")
		}
		return
	}
	pRaw, hasP := m["phases"]
	if !hasP {
		return
	}
	pm, pok := decodeObject(pRaw)
	if !pok {
		return
	}
	touched := false
	for _, k := range PhaseSet {
		v, ok := pm[k]
		if !ok || strings.TrimSpace(string(v)) == "null" {
			continue
		}
		if f, fok := asFloat(v); fok {
			n := ClampInt(int(f+0.5), 60, 180*60)
			if float64(n) != f {
				pm[k] = mustMarshal(n)
				*changes = append(*changes, fmt.Sprintf("M6 阶段 %s 时长钳制为 %ds", k, n))
				touched = true
			}
		}
	}
	if touched {
		m["phases"] = mustMarshal(pm)
		data[KeyTimerPrefs] = mustMarshal(m)
	}
}
