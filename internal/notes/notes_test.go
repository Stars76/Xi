package notes

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Stars76/526Hz/internal/store"
)

func TestRenderAndBuildNote(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir, store.NewLogger(filepath.Join(dir, "logs")))
	// Cancel the debounced disk flush before TempDir cleanup runs, otherwise a late write races the removal.
	t.Cleanup(func() { _ = st.Flush() })
	st.Load()

	day := "2026-09-09"
	st.SetJSON(store.KeyStateV2, store.FocusState{
		ViewDay: day,
		Days: map[string]*store.DayBucket{
			day: {
				Tasks: []store.Task{{ID: "t1", Title: "完成重构", Done: true, Subtasks: []store.Subtask{}}},
				DismissedDaily: map[string]bool{},
			},
		},
	})
	st.SetJSON(store.KeyTimerHistory, []store.HistoryEntry{
		{
			Day: day, Phase: "focus", Result: "done",
			StartedAt:  time.Date(2026, 9, 9, 9, 0, 0, 0, time.Local).Format(time.RFC3339),
			FinishedAt: time.Date(2026, 9, 9, 9, 25, 0, 0, time.Local).Format(time.RFC3339),
			PlannedMinutes: 25, ActualMinutes: 25, TaskID: "t1", TaskDay: day, TaskTitle: "完成重构",
		},
	})

	now := time.Date(2026, 9, 9, 15, 0, 0, 0, time.Local)
	daily := BuildNote(st, BuildRequest{Kind: "daily", Day: day}, now)
	if !daily.OK || !strings.Contains(daily.Text, "完成重构") || !strings.Contains(daily.Text, "专注 25 分钟") {
		t.Fatalf("unexpected daily note: %+v", daily)
	}
	if strings.Contains(daily.Text, "今天还没有完成的任务") {
		t.Fatalf("empty block should not render when list is non-empty: %s", daily.Text)
	}

	// Unknown variables preserved verbatim
	custom := Render("hi {{name}} {{unknown_var}}", RenderData{Vars: map[string]string{"name": "526Hz"}})
	if custom != "hi 526Hz {{unknown_var}}" {
		t.Fatalf("expected unknown var preserved, got %q", custom)
	}
}
