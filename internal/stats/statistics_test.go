package stats

import (
	"testing"
	"time"

	"github.com/Stars76/Xi/internal/store"
)

func localISO(y int, m time.Month, d, hh, mm int) string {
	return time.Date(y, m, d, hh, mm, 0, 0, time.Local).Format(time.RFC3339)
}

func TestDayTimelineMergeOverridesAndLayout(t *testing.T) {
	day := "2026-09-09"
	h := []store.HistoryEntry{
		{
			Day: day, Phase: "focus", Result: "done",
			StartedAt: localISO(2026, 9, 9, 8, 0), FinishedAt: localISO(2026, 9, 9, 8, 25),
			PlannedMinutes: 25, ActualMinutes: 25, TaskID: "t1", TaskDay: day, TaskTitle: "写文档",
		},
		{
			Day: day, Phase: "short", Result: "done",
			StartedAt: localISO(2026, 9, 9, 8, 25), FinishedAt: localISO(2026, 9, 9, 8, 30),
			PlannedMinutes: 5, ActualMinutes: 5, TaskID: "t1", TaskDay: day, TaskTitle: "写文档",
		},
		{
			Day: day, Phase: "focus", Result: "done",
			StartedAt: localISO(2026, 9, 9, 8, 30), FinishedAt: localISO(2026, 9, 9, 8, 55),
			PlannedMinutes: 25, ActualMinutes: 25, TaskID: "t1", TaskDay: day, TaskTitle: "写文档",
		},
		{
			Day: day, Phase: "focus", Result: "abandoned",
			StartedAt: localISO(2026, 9, 9, 9, 10), FinishedAt: localISO(2026, 9, 9, 9, 15),
			PlannedMinutes: 25, ActualMinutes: 5, TaskID: "t1", TaskDay: day, TaskTitle: "写文档",
		},
		{
			Day: day, Phase: "focus", Result: "done",
			StartedAt: localISO(2026, 9, 9, 10, 0), FinishedAt: localISO(2026, 9, 9, 10, 25),
			PlannedMinutes: 25, ActualMinutes: 25,
		},
	}

	blocks := DayTimeline(h, day, nil)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks (merged, abandoned, unbound), got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Minutes != 50 || blocks[0].Segments != 2 {
		t.Fatalf("expected merged 50m / 2 segs, got %+v", blocks[0])
	}
	if blocks[1].Result != "abandoned" {
		t.Fatalf("expected abandoned block second, got %+v", blocks[1])
	}
	if blocks[2].Title != "未定任务的专注" {
		t.Fatalf("expected unbound title, got %q", blocks[2].Title)
	}

	hiddenFlag := true
	ov := &store.TimelineOverrides{
		Edits: []store.TimelineEdit{{Key: blocks[1].Key, Hidden: &hiddenFlag}},
		Manual: []store.ManualBlock{
			{ID: "m1", Day: day, Start: "08:10", End: "09:00", Title: "开会"},
		},
	}
	withOv := DayTimeline(h, day, ov)
	if len(withOv) != 3 {
		t.Fatalf("expected 3 visible blocks with manual overlap, got %d", len(withOv))
	}
	laid := LayoutTimeline(withOv)
	if laid[0].Cols != 2 || laid[1].Cols != 2 {
		t.Fatalf("expected overlapping blocks to split into 2 cols, got %+v", laid[:2])
	}
	hk := HiddenTimelineKeys(h, day, ov)
	if len(hk) != 1 || hk[0] != blocks[1].Key {
		t.Fatalf("expected 1 hidden key, got %v", hk)
	}

	// Summary check
	sum := Summary(h, map[string]*store.DayBucket{
		day: {Tasks: []store.Task{{ID: "t1", Title: "写文档", Done: true}}},
	}, "week", time.Date(2026, 9, 9, 12, 0, 0, 0, time.Local))
	if sum.Minutes != 75 || sum.Rounds != 3 || sum.Tasks != 1 || sum.Streak != 1 {
		t.Fatalf("unexpected weekly summary: %+v", sum)
	}
}

func TestLayoutTimelineVisualMinSpanNoOverlap(t *testing.T) {
	day := "2026-09-09"
	h := []store.HistoryEntry{
		{Day: day, Phase: "focus", Result: "abandoned", StartedAt: localISO(2026, 9, 9, 9, 0), FinishedAt: localISO(2026, 9, 9, 9, 2), PlannedMinutes: 25, ActualMinutes: 2},
		{Day: day, Phase: "focus", Result: "abandoned", StartedAt: localISO(2026, 9, 9, 9, 6), FinishedAt: localISO(2026, 9, 9, 9, 8), PlannedMinutes: 25, ActualMinutes: 2},
		{Day: day, Phase: "focus", Result: "done", StartedAt: localISO(2026, 9, 9, 9, 14), FinishedAt: localISO(2026, 9, 9, 9, 39), PlannedMinutes: 25, ActualMinutes: 25, TaskID: "t1", TaskDay: day, TaskTitle: "正常专注"},
		// Standard 25m + 5m break + 25m on different tasks: should stay cleanly in 1 column
		{Day: day, Phase: "focus", Result: "done", StartedAt: localISO(2026, 9, 9, 11, 0), FinishedAt: localISO(2026, 9, 9, 11, 25), PlannedMinutes: 25, ActualMinutes: 25, TaskID: "t2", TaskDay: day, TaskTitle: "任务A"},
		{Day: day, Phase: "focus", Result: "done", StartedAt: localISO(2026, 9, 9, 11, 30), FinishedAt: localISO(2026, 9, 9, 11, 55), PlannedMinutes: 25, ActualMinutes: 25, TaskID: "t3", TaskDay: day, TaskTitle: "任务B"},
	}

	raw := DayTimeline(h, day, nil)
	if len(raw) != 5 {
		t.Fatalf("expected 5 blocks, got %d", len(raw))
	}
	if raw[0].Minutes != 2 {
		t.Fatalf("expected abandoned block minutes=2 (actual), got %d", raw[0].Minutes)
	}

	laid := LayoutTimelineWithMinSpan(raw, 29)
	// First 3 nearby blocks (09:00, 09:06, 09:14) must split into 3 side-by-side columns
	for i := 0; i < 3; i++ {
		if laid[i].Cols != 3 || laid[i].Col != i {
			t.Fatalf("block %d expected Col=%d Cols=3, got %+v", i, i, laid[i])
		}
	}
	// 11:00-11:25 and 11:30-11:55 (30m apart >= 29m minVisualMin) stay in 1 full-width column
	if laid[3].Cols != 1 || laid[4].Cols != 1 {
		t.Fatalf("expected spaced pomodoros in 1 column, got %+v %+v", laid[3], laid[4])
	}
}

func TestYearlySummaryAndVoyageLine(t *testing.T) {
	now := time.Date(2026, 9, 9, 14, 0, 0, 0, time.Local)
	h := []store.HistoryEntry{
		{Day: "2026-09-07", Phase: "focus", Result: "done", PlannedMinutes: 25},
		{Day: "2026-09-08", Phase: "focus", Result: "done", PlannedMinutes: 65},
		{Day: "2026-09-09", Phase: "focus", Result: "done", PlannedMinutes: 120},
		{Day: "2026-09-09", Phase: "focus", Result: "abandoned", PlannedMinutes: 25, ActualMinutes: 3},
	}
	days := map[string]*store.DayBucket{
		"2026-09-09": {Tasks: []store.Task{{ID: "t1", Title: "收尾", Done: true}, {ID: "t2", Title: "待办", Done: false}}},
	}
	ys := YearlySummary(h, days, now)
	if ys.TotalMinutes != 210 || ys.TotalRounds != 3 || ys.ActiveDays != 3 || ys.LongestStreak != 3 || ys.CurrentStreak != 3 || ys.TotalTasks != 1 {
		t.Fatalf("unexpected yearly summary: %+v", ys)
	}
	if len(ys.Days) < 358 || ys.Days[len(ys.Days)-1].Level != 4 {
		t.Fatalf("expected 52-week grid ending at level 4, len=%d last=%+v", len(ys.Days), ys.Days[len(ys.Days)-1])
	}
	line := VoyageSummaryLine("今天", "细雨", 120, 4, 3, 2, "深海蓝鲸 · DeepSeek")
	if line == "" {
		t.Fatal("expected non-empty voyage summary line")
	}
}
