package stats

import (
	"fmt"
	"sort"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/store"
)

const MergeGap = 15 * time.Minute

type PeriodSummary struct {
	Since     string
	Until     string
	Minutes   int
	Rounds    int
	Tasks     int
	Streak    int
	Hours     [24]int
	Bins      [12]int
	PeakIndex int
	Peak      int
}

type TimelineBlock struct {
	Key            string
	ID             string
	TaskID         string
	TaskDay        string
	Title          string
	Result         string // done | recovered | abandoned | manual
	Segments       int
	Minutes        int
	StartMin       int
	EndMin         int
	Edited         bool
	Hidden         bool
	Col            int
	Cols           int
	HistoryIndices []int
	endTime        time.Time
}

func isCompletedFocus(h store.HistoryEntry) bool {
	return h.Phase == "focus" && (h.Result == "done" || h.Result == "recovered")
}

func Summary(history []store.HistoryEntry, days map[string]*store.DayBucket, period string, now time.Time) PeriodSummary {
	now = now.Local()
	today := dateutil.TodayKey(now)
	start := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	if period == "month" {
		start = time.Date(now.Year(), now.Month(), 1, 12, 0, 0, 0, time.Local)
	} else {
		wd := (int(start.Weekday()) + 6) % 7 // Monday = 0
		start = start.AddDate(0, 0, -wd)
	}
	since := start.Format("2006-01-02")

	var out PeriodSummary
	out.Since = since
	out.Until = today
	active := map[string]bool{}

	for _, h := range history {
		if !isCompletedFocus(h) {
			continue
		}
		fin, finErr := time.Parse(time.RFC3339Nano, h.FinishedAt)
		if finErr != nil {
			fin, finErr = time.Parse(time.RFC3339, h.FinishedAt)
		}
		day := h.Day
		if !dateutil.IsValidKey(day) {
			if finErr == nil {
				day = dateutil.TodayKey(fin)
			} else {
				day = ""
			}
		}
		if day == "" || day > today || (finErr == nil && fin.After(now)) {
			continue
		}
		active[day] = true
		if day < since {
			continue
		}
		credited := max(0, h.PlannedMinutes)
		out.Minutes += credited
		out.Rounds++
		at := fin
		if beg, berr := time.Parse(time.RFC3339Nano, h.StartedAt); berr == nil {
			at = beg
		} else if beg2, berr2 := time.Parse(time.RFC3339, h.StartedAt); berr2 == nil {
			at = beg2
		}
		if !at.IsZero() {
			out.Hours[at.Local().Hour()] += credited
		}
	}

	for day, bucket := range days {
		if !dateutil.IsValidKey(day) || day < since || day > today || bucket == nil {
			continue
		}
		for _, t := range bucket.Tasks {
			if t.Done {
				out.Tasks++
			}
		}
	}

	cursor := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	if !active[cursor.Format("2006-01-02")] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for active[cursor.Format("2006-01-02")] {
		out.Streak++
		cursor = cursor.AddDate(0, 0, -1)
	}

	out.PeakIndex = -1
	for i := 0; i < 12; i++ {
		out.Bins[i] = out.Hours[i*2] + out.Hours[i*2+1]
		if out.Bins[i] > out.Peak {
			out.Peak = out.Bins[i]
			out.PeakIndex = i
		}
	}
	return out
}

func TaskTotals(history []store.HistoryEntry) map[string]int {
	totals := map[string]int{}
	for _, h := range history {
		if !isCompletedFocus(h) || h.TaskID == "" || !dateutil.IsValidKey(h.TaskDay) {
			continue
		}
		m := max(0, h.PlannedMinutes)
		totals[h.TaskDay+"/"+h.TaskID] += m
		totals[h.TaskID] += m
	}
	return totals
}

func ToMin(hhmm string) (int, bool) {
	if !dateutil.IsValidAlarm(hhmm) {
		return 0, false
	}
	var h, m int
	_, err := fmt.Sscanf(hhmm, "%02d:%02d", &h, &m)
	if err != nil {
		return 0, false
	}
	return h*60 + m, true
}

func ToHhmm(min int) string {
	min = store.ClampInt(min, 0, 1440)
	h := min / 60
	m := min % 60
	if h == 24 {
		return "24:00"
	}
	return fmt.Sprintf("%02d:%02d", h, m)
}

func parseISO(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Local(), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Local(), true
	}
	return time.Time{}, false
}

func blockMins(start, end time.Time) (int, int) {
	sm := start.Hour()*60 + start.Minute()
	em := end.Hour()*60 + end.Minute()
	if end.After(start) && end.Day() != start.Day() {
		em += 1440
	}
	if em < sm+1 {
		em = sm + 1
	}
	if em > 1440 {
		em = 1440
	}
	return sm, em
}

func makeBlock(h store.HistoryEntry, start, end time.Time, focus bool) TimelineBlock {
	day := h.Day
	if !dateutil.IsValidKey(day) {
		day = dateutil.TodayKey(end)
	}
	sm, em := blockMins(start, end)
	taskTag := h.TaskID
	if taskTag == "" {
		taskTag = "~"
	}
	tsTag := h.StartedAt
	if tsTag == "" {
		tsTag = h.FinishedAt
	}
	title := h.TaskTitle
	if title == "" {
		if h.TaskID != "" {
			title = "（未命名任务）"
		} else {
			title = "未定任务的专注"
		}
	}
	segs, mins := 0, 0
	if focus {
		segs = 1
		if h.Result == "abandoned" {
			mins = max(0, h.ActualMinutes)
		} else {
			mins = max(0, h.PlannedMinutes)
		}
	}
	res := h.Result
	if res == "" {
		res = "done"
	}
	return TimelineBlock{
		Key:      day + "|" + taskTag + "|" + tsTag,
		TaskID:   h.TaskID,
		TaskDay:  dateutil.NormalizeKey(h.TaskDay),
		Title:    title,
		Result:   res,
		Segments: segs,
		Minutes:  mins,
		StartMin: sm,
		EndMin:   em,
		endTime:  end,
	}
}

func deriveDayBlocks(history []store.HistoryEntry, day string) []TimelineBlock {
	if !dateutil.IsValidKey(day) {
		return nil
	}
	type seg struct {
		idx   int
		h     store.HistoryEntry
		start time.Time
		end   time.Time
	}
	var segs []seg
	for i, h := range history {
		if h.Day != day {
			continue
		}
		fin, ok := parseISO(h.FinishedAt)
		if !ok {
			continue
		}
		st, sok := parseISO(h.StartedAt)
		if !sok {
			st = fin.Add(-time.Duration(max(0, h.ActualMinutes)) * time.Minute)
		}
		segs = append(segs, seg{idx: i, h: h, start: st, end: fin})
	}
	sort.Slice(segs, func(i, j int) bool {
		if !segs[i].start.Equal(segs[j].start) {
			return segs[i].start.Before(segs[j].start)
		}
		return segs[i].end.Before(segs[j].end)
	})

	var blocks []TimelineBlock
	var open *TimelineBlock
	flush := func() {
		if open != nil {
			blocks = append(blocks, *open)
			open = nil
		}
	}
	for _, s := range segs {
		focus := s.h.Phase == "focus"
		merged := s.h.TaskID != "" && s.h.Result != "abandoned"
		if !merged {
			flush()
			b := makeBlock(s.h, s.start, s.end, focus)
			b.HistoryIndices = []int{s.idx}
			blocks = append(blocks, b)
			continue
		}
		if open != nil && open.TaskID == s.h.TaskID {
			gap := s.start.Sub(open.endTime)
			if gap >= 0 && gap <= MergeGap {
				open.endTime = s.end
				_, em := blockMins(s.start, s.end)
				if em > open.EndMin {
					open.EndMin = em
				}
				if focus {
					open.Segments++
					open.Minutes += max(0, s.h.PlannedMinutes)
				}
				open.HistoryIndices = append(open.HistoryIndices, s.idx)
				continue
			}
		}
		flush()
		b := makeBlock(s.h, s.start, s.end, focus)
		b.HistoryIndices = []int{s.idx}
		open = &b
	}
	flush()

	out := make([]TimelineBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Segments > 0 {
			out = append(out, b)
		}
	}
	return out
}

func DayTimeline(history []store.HistoryEntry, day string, overrides *store.TimelineOverrides) []TimelineBlock {
	if !dateutil.IsValidKey(day) {
		return nil
	}
	return ApplyTimelineOverrides(deriveDayBlocks(history, day), overrides, day)
}

func HiddenTimelineKeys(history []store.HistoryEntry, day string, overrides *store.TimelineOverrides) []string {
	if !dateutil.IsValidKey(day) || overrides == nil || len(overrides.Edits) == 0 {
		return nil
	}
	hidden := map[string]bool{}
	for _, e := range overrides.Edits {
		if e.Key != "" && e.Hidden != nil && *e.Hidden {
			hidden[e.Key] = true
		}
	}
	var keys []string
	for _, b := range deriveDayBlocks(history, day) {
		if hidden[b.Key] {
			keys = append(keys, b.Key)
		}
	}
	return keys
}

func ApplyTimelineOverrides(blocks []TimelineBlock, overrides *store.TimelineOverrides, day string) []TimelineBlock {
	out := make([]TimelineBlock, len(blocks))
	copy(out, blocks)

	if overrides != nil {
		for _, e := range overrides.Edits {
			if e.Key == "" {
				continue
			}
			for i := range out {
				if out[i].Key != e.Key {
					continue
				}
				if e.Start != "" {
					if m, ok := ToMin(e.Start); ok {
						out[i].StartMin = m
						out[i].Edited = true
					}
				}
				if e.End != "" {
					if m, ok := ToMin(e.End); ok {
						out[i].EndMin = max(m, out[i].StartMin+1)
						out[i].Edited = true
					}
				}
				if e.Hidden != nil && *e.Hidden {
					out[i].Hidden = true
				}
			}
		}
		for _, m := range overrides.Manual {
			if m.Day != day {
				continue
			}
			s, ok1 := ToMin(m.Start)
			e, ok2 := ToMin(m.End)
			if !ok1 || !ok2 || e <= s {
				continue
			}
			title := m.Title
			if title == "" {
				title = "手动补录"
			}
			out = append(out, TimelineBlock{
				Key:      "manual|" + m.ID,
				ID:       m.ID,
				Title:    title,
				Result:   "manual",
				Segments: 1,
				Minutes:  e - s,
				StartMin: s,
				EndMin:   e,
			})
		}
	}

	visible := make([]TimelineBlock, 0, len(out))
	for _, b := range out {
		if !b.Hidden {
			visible = append(visible, b)
		}
	}
	sort.Slice(visible, func(i, j int) bool {
		if visible[i].StartMin != visible[j].StartMin {
			return visible[i].StartMin < visible[j].StartMin
		}
		return visible[i].EndMin < visible[j].EndMin
	})
	return visible
}

// LayoutTimeline assigns Col and Cols for overlapping blocks side by side using exact logical EndMin.
func LayoutTimeline(blocks []TimelineBlock) []TimelineBlock {
	return LayoutTimelineWithMinSpan(blocks, 0)
}

// LayoutTimelineWithMinSpan assigns Col and Cols considering a minimum visual span (in minutes)
// so short or nearby cards with a minimum pixel height never overlap vertically.
func LayoutTimelineWithMinSpan(blocks []TimelineBlock, minVisualMin int) []TimelineBlock {
	list := make([]TimelineBlock, len(blocks))
	copy(list, blocks)
	var cluster []int
	var cols []int
	clusterEnd := -1

	visualEnd := func(b TimelineBlock) int {
		end := b.EndMin
		if minVisualMin > 0 && b.StartMin+minVisualMin > end {
			end = b.StartMin + minVisualMin
		}
		return end
	}

	flushCluster := func() {
		total := max(1, len(cols))
		for _, idx := range cluster {
			list[idx].Cols = total
		}
		cluster = nil
		cols = nil
		clusterEnd = -1
	}

	for i := range list {
		if len(cluster) > 0 && list[i].StartMin >= clusterEnd {
			flushCluster()
		}
		vEnd := visualEnd(list[i])
		placed := false
		for c := range cols {
			if cols[c] <= list[i].StartMin {
				cols[c] = vEnd
				list[i].Col = c
				placed = true
				break
			}
		}
		if !placed {
			list[i].Col = len(cols)
			cols = append(cols, vEnd)
		}
		cluster = append(cluster, i)
		if vEnd > clusterEnd {
			clusterEnd = vEnd
		}
	}
	flushCluster()
	return list
}

// YearHeatmapDay represents one cell in the 52-week ocean-tide heatmap.
type YearHeatmapDay struct {
	Day     string
	Minutes int
	Rounds  int
	Level   int // 0..4 intensity scale
}

// YearSummary aggregates 52 weeks of focus activity for the Yearly Tide Heatmap tab.
type YearSummary struct {
	Since         string
	Until         string
	TotalMinutes  int
	TotalRounds   int
	TotalTasks    int
	ActiveDays    int
	CurrentStreak int
	LongestStreak int
	MaxDayMinutes int
	Weeks         int
	Days          []YearHeatmapDay
}

// YearlySummary computes a Monday-aligned 52-week focus heatmap ending on today's week.
func YearlySummary(history []store.HistoryEntry, days map[string]*store.DayBucket, now time.Time) YearSummary {
	now = now.Local()
	today := dateutil.TodayKey(now)
	todayNoon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
	// Align start to Monday 51 weeks before this week's Monday (52 full/partial columns)
	wd := (int(todayNoon.Weekday()) + 6) % 7 // Monday=0 .. Sunday=6
	thisMon := todayNoon.AddDate(0, 0, -wd)
	start := thisMon.AddDate(0, 0, -51*7)
	since := start.Format("2006-01-02")

	minByDay := map[string]int{}
	roundByDay := map[string]int{}
	for _, h := range history {
		if !isCompletedFocus(h) {
			continue
		}
		day := h.Day
		if !dateutil.IsValidKey(day) {
			if fin, ok := parseISO(h.FinishedAt); ok {
				day = dateutil.TodayKey(fin)
			}
		}
		if day == "" || day > today {
			continue
		}
		minByDay[day] += max(0, h.PlannedMinutes)
		roundByDay[day]++
	}

	out := YearSummary{
		Since: since,
		Until: today,
		Weeks: 52,
	}

	// Count completed tasks in range
	for dk, b := range days {
		if !dateutil.IsValidKey(dk) || dk < since || dk > today || b == nil {
			continue
		}
		for _, t := range b.Tasks {
			if t.Done {
				out.TotalTasks++
			}
		}
	}

	// Walk all days from start..today
	curStreak := 0
	for cur := start; !cur.After(todayNoon); cur = cur.AddDate(0, 0, 1) {
		dk := cur.Format("2006-01-02")
		m := minByDay[dk]
		r := roundByDay[dk]
		if m > 0 {
			out.TotalMinutes += m
			out.TotalRounds += r
			out.ActiveDays++
			if m > out.MaxDayMinutes {
				out.MaxDayMinutes = m
			}
			curStreak++
			if curStreak > out.LongestStreak {
				out.LongestStreak = curStreak
			}
		} else {
			curStreak = 0
		}
		lvl := 0
		switch {
		case m >= 120:
			lvl = 4
		case m >= 60:
			lvl = 3
		case m >= 25:
			lvl = 2
		case m > 0:
			lvl = 1
		}
		out.Days = append(out.Days, YearHeatmapDay{
			Day:     dk,
			Minutes: m,
			Rounds:  r,
			Level:   lvl,
		})
	}

	// Current streak ending today or yesterday
	cursor := todayNoon
	if minByDay[cursor.Format("2006-01-02")] == 0 {
		cursor = cursor.AddDate(0, 0, -1)
	}
	for minByDay[cursor.Format("2006-01-02")] > 0 {
		out.CurrentStreak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return out
}

// VoyageSummaryLine builds a warm, concise 1-line sailing reflection for any statistics period.
func VoyageSummaryLine(scopeLabel, weatherLabel string, minutes, rounds, tasksDone, sightings int, highlight string) string {
	if minutes == 0 && tasksDone == 0 {
		return fmt.Sprintf("%s海面宁静如初，开启一轮专注起潮吧。", scopeLabel)
	}
	prefix := scopeLabel
	if weatherLabel != "" {
		prefix = fmt.Sprintf("%s在%s中", scopeLabel, weatherLabel)
	}
	parts := ""
	if minutes > 0 {
		parts = fmt.Sprintf("专注了 %d 分钟（%d 轮）", minutes, rounds)
	}
	if tasksDone > 0 {
		if parts != "" {
			parts += "，"
		}
		parts += fmt.Sprintf("完成 %d 项计划", tasksDone)
	}
	if highlight != "" {
		parts += fmt.Sprintf("，曾偶遇「%s」", highlight)
	} else if sightings > 0 {
		parts += fmt.Sprintf("，累计见证 %d 次海面奇遇", sightings)
	}
	return prefix + parts + "。"
}
