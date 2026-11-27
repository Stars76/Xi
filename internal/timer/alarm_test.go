package timer

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Stars76/526Hz/internal/store"
)

func TestAlarmSchedulerDedupeAndReset(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir, store.NewLogger(filepath.Join(dir, "logs")))
	st.Load()

	now := time.Date(2026, 9, 9, 8, 29, 0, 0, time.Local)
	var fired []string
	sched := NewAlarmScheduler(st, nil, func(title, body string) {
		fired = append(fired, title)
	}, func() time.Time { return now })

	st.SetJSON(store.KeyStateV2, store.FocusState{
		ViewDay: "2026-09-09",
		Days: map[string]*store.DayBucket{
			"2026-09-09": {
				Tasks: []store.Task{
					{ID: "a", Title: "开会", Alarm: "08:30", Done: false, Subtasks: []store.Subtask{}},
					{ID: "b", Title: "已完成", Alarm: "08:30", Done: true, Subtasks: []store.Subtask{}},
				},
				DismissedDaily: map[string]bool{},
			},
		},
	})

	sched.Tick()
	if len(fired) != 0 {
		t.Fatalf("expected no alarm at 08:29, got %v", fired)
	}

	now = time.Date(2026, 9, 9, 8, 30, 10, 0, time.Local)
	sched.Tick()
	sched.Tick() // same minute dedupe
	if len(fired) != 1 || fired[0] != "⏰ 闹钟：开会" {
		t.Fatalf("expected single alarm for 开会, got %v", fired)
	}

	// Next day resets dedupe set
	now = time.Date(2026, 9, 10, 8, 30, 0, 0, time.Local)
	st.SetJSON(store.KeyStateV2, store.FocusState{
		ViewDay: "2026-09-10",
		Days: map[string]*store.DayBucket{
			"2026-09-10": {
				Tasks: []store.Task{
					{ID: "a", Title: "开会", Alarm: "08:30", Done: false, Subtasks: []store.Subtask{}},
				},
				DismissedDaily: map[string]bool{},
			},
		},
	})
	sched.Tick()
	if len(fired) != 2 {
		t.Fatalf("expected alarm to fire again next day, got %v", fired)
	}
}
