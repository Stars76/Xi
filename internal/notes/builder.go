package notes

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Stars76/526Hz/internal/dateutil"
	"github.com/Stars76/526Hz/internal/stats"
	"github.com/Stars76/526Hz/internal/store"
)

type BuildRequest struct {
	Kind         string // daily | task | weekly
	Day          string
	TaskID       string
	TaskDay      string
	TemplateText string // optional live preview override
}

type BuildResult struct {
	OK       bool
	Text     string
	Name     string
	FileName string
	Error    string
}

func slugTitle(s string) string {
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_", " ", "_", "\t", "_", "\n", "_")
	out := replacer.Replace(s)
	r := []rune(out)
	if len(r) > 24 {
		out = string(r[:24])
	}
	return out
}

func BuildNote(st *store.Store, req BuildRequest, now ...time.Time) BuildResult {
	if req.Kind != "daily" && req.Kind != "task" && req.Kind != "weekly" {
		return BuildResult{Error: "未知笔记类型"}
	}
	curNow := time.Now()
	if len(now) > 0 && !now[0].IsZero() {
		curNow = now[0]
	}

	var fs store.FocusState
	_ = st.GetJSON(store.KeyStateV2, &fs)
	days := fs.Days
	if days == nil {
		days = map[string]*store.DayBucket{}
	}
	var history []store.HistoryEntry
	_ = st.GetJSON(store.KeyTimerHistory, &history)
	var ov store.TimelineOverrides
	hasOv := st.GetJSON(store.KeyTimelineOverrides, &ov)
	var ovPtr *store.TimelineOverrides
	if hasOv {
		ovPtr = &ov
	}
	var storedTpl store.ExportTemplate
	hasTpl := st.GetJSON(store.KeyExportTemplate, &storedTpl)
	var tplPtr *store.ExportTemplate
	if hasTpl {
		tplPtr = &storedTpl
	}

	totals := stats.TaskTotals(history)
	tpl := Pick(tplPtr, req.Kind)
	vars := map[string]string{}
	lists := map[string][]map[string]string{}
	toRows := func(blocks []stats.TimelineBlock) []map[string]string {
		rows := make([]map[string]string, 0, len(blocks))
		for _, b := range blocks {
			rows = append(rows, map[string]string{
				"start":   stats.ToHhmm(b.StartMin),
				"end":     stats.ToHhmm(b.EndMin),
				"title":   b.Title,
				"minutes": fmt.Sprint(b.Minutes),
			})
		}
		return rows
	}

	var fileName string
	switch req.Kind {
	case "daily":
		dayKey := dateutil.NormalizeKey(req.Day)
		if dayKey == "" {
			dayKey = dateutil.TodayKey(curNow)
		}
		blocks := stats.DayTimeline(history, dayKey, ovPtr)
		var dayTasks []store.Task
		if b := days[dayKey]; b != nil {
			dayTasks = b.Tasks
		}
		var done []store.Task
		focusMins, focusRounds := 0, 0
		for _, b := range blocks {
			focusMins += b.Minutes
			focusRounds += b.Segments
		}
		for _, t := range dayTasks {
			if t.Done {
				done = append(done, t)
			}
		}
		vars["date"] = dayKey
		vars["date_cn"] = dateutil.DayLabel(dayKey, curNow.Year())
		vars["weekday"] = dateutil.WeekdayLabel(dayKey)
		vars["focus_minutes"] = fmt.Sprint(focusMins)
		vars["focus_rounds"] = fmt.Sprint(focusRounds)
		vars["tasks_done_count"] = fmt.Sprint(len(done))
		vars["tasks_total_count"] = fmt.Sprint(len(dayTasks))
		lists["timeline"] = toRows(blocks)
		doneRows := make([]map[string]string, 0, len(done))
		for _, t := range done {
			title := t.Title
			if title == "" {
				title = "（未命名任务）"
			}
			doneRows = append(doneRows, map[string]string{
				"title":   title,
				"minutes": fmt.Sprint(totals[t.ID]),
			})
		}
		lists["tasks_done"] = doneRows
		fileName = "每日复盘-" + dayKey + ".md"

	case "task":
		if req.TaskID == "" || len(req.TaskID) > 128 {
			return BuildResult{Error: "缺少任务标识"}
		}
		dayKey := dateutil.NormalizeKey(req.TaskDay)
		if dayKey == "" {
			return BuildResult{Error: "任务日期非法"}
		}
		title := ""
		found := false
		if b := days[dayKey]; b != nil {
			for _, t := range b.Tasks {
				if t.ID == req.TaskID {
					title = t.Title
					found = true
					break
				}
			}
		}
		if !found {
			for _, h := range history {
				if h.TaskID == req.TaskID && h.TaskTitle != "" {
					title = h.TaskTitle
					found = true
				}
			}
		}
		if !found {
			return BuildResult{Error: "任务不存在"}
		}
		allBlocks := stats.DayTimeline(history, dayKey, ovPtr)
		var blocks []stats.TimelineBlock
		focusMins, focusRounds := 0, 0
		for _, b := range allBlocks {
			if b.TaskID == req.TaskID {
				blocks = append(blocks, b)
				focusMins += b.Minutes
				focusRounds += b.Segments
			}
		}
		displayTitle := title
		if displayTitle == "" {
			displayTitle = "（未命名任务）"
		}
		vars["task_title"] = displayTitle
		vars["task_id"] = req.TaskID
		vars["date"] = dayKey
		vars["date_cn"] = dateutil.DayLabel(dayKey, curNow.Year())
		vars["task_minutes"] = fmt.Sprint(totals[req.TaskID])
		vars["focus_minutes"] = fmt.Sprint(focusMins)
		vars["focus_rounds"] = fmt.Sprint(focusRounds)
		lists["timeline"] = toRows(blocks)
		slug := slugTitle(title)
		if slug == "" {
			slug = req.TaskID[:min(8, len(req.TaskID))]
		}
		fileName = fmt.Sprintf("任务-%s-%s.md", slug, dayKey)

	case "weekly":
		s := stats.Summary(history, days, "week", curNow)
		dayKeys := make([]string, 0, len(days))
		for k := range days {
			if k >= s.Since && k <= s.Until {
				dayKeys = append(dayKeys, k)
			}
		}
		slices.Sort(dayKeys)
		var doneRows []map[string]string
		for _, k := range dayKeys {
			if b := days[k]; b != nil {
				for _, t := range b.Tasks {
					if t.Done {
						title := t.Title
						if title == "" {
							title = "（未命名任务）"
						}
						doneRows = append(doneRows, map[string]string{
							"title":   title,
							"minutes": fmt.Sprint(totals[t.ID]),
						})
					}
				}
			}
		}
		peakRange := "暂无"
		if s.PeakIndex >= 0 {
			peakRange = fmt.Sprintf("%d–%d 点", s.PeakIndex*2, s.PeakIndex*2+2)
		}
		vars["week_since"] = s.Since
		vars["week_until"] = s.Until
		vars["date"] = s.Until
		vars["date_cn"] = dateutil.DayLabel(s.Until, curNow.Year())
		vars["focus_minutes"] = fmt.Sprint(s.Minutes)
		vars["focus_rounds"] = fmt.Sprint(s.Rounds)
		vars["tasks_done_count"] = fmt.Sprint(s.Tasks)
		vars["streak"] = fmt.Sprint(s.Streak)
		vars["peak_range"] = peakRange
		lists["tasks_done"] = doneRows
		fileName = "周报-" + s.Since + ".md"
	}

	text := tpl.Text
	if req.TemplateText != "" && len(req.TemplateText) <= store.MaxTemplateLen {
		text = req.TemplateText
	}
	return BuildResult{
		OK:       true,
		Text:     Render(text, RenderData{Vars: vars, Lists: lists}),
		Name:     tpl.Name,
		FileName: fileName,
	}
}
