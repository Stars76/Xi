package todo

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Stars76/526Hz/internal/store"
)

func TestRolloverDailyAndSubtasks(t *testing.T) {
	dir := t.TempDir()
	st := store.NewStore(dir, store.NewLogger(filepath.Join(dir, "logs")))
	st.Load()

	now := time.Date(2026, 9, 10, 10, 0, 0, 0, time.Local)
	st.SetJSON(store.KeyStateV2, store.FocusState{
		ViewDay: "2026-09-10",
		Days: map[string]*store.DayBucket{
			"2026-09-09": {
				Tasks: []store.Task{
					{ID: "open1", Title: "未完成普通任务", Done: false, Subtasks: []store.Subtask{}},
					{ID: "done1", Title: "已完成普通任务", Done: true, Subtasks: []store.Subtask{}},
					{ID: "daily1", Title: "每日饮水", DailyID: "d1", Done: false, Subtasks: []store.Subtask{}},
				},
				DismissedDaily: map[string]bool{},
			},
			"2026-09-10": {
				Tasks: []store.Task{
					{ID: "today1", Title: "今天任务", Done: false, Subtasks: []store.Subtask{}},
				},
				DismissedDaily: map[string]bool{},
			},
		},
	})
	st.SetJSON(store.KeyDaily, store.DailyConfig{
		Tasks: []store.DailyTemplate{{ID: "d1", Title: "每日饮水"}},
	})

	svc := NewService(st, nil, func() time.Time { return now })
	svc.Init()

	tasks := svc.Tasks()
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks (rolled, today, daily), got %+v", tasks)
	}
	if tasks[0].ID != "open1" || tasks[0].RolledFrom != "2026-09-09" {
		t.Fatalf("expected rolled task at top with rolledFrom=2026-09-09, got %+v", tasks[0])
	}
	// Idempotent second rollover
	if svc.RolloverOpenTasks() {
		t.Fatal("second rollover must be idempotent")
	}

	// Delete daily task -> dismissedDaily prevents re-materialization today
	var dailyTaskID string
	for _, tk := range svc.Tasks() {
		if tk.DailyID == "d1" {
			dailyTaskID = tk.ID
		}
	}
	removed, idx, ok := svc.DeleteTask(dailyTaskID)
	if !ok {
		t.Fatal("delete daily task failed")
	}
	if svc.SyncDaily() {
		t.Fatal("dismissed daily task must not reappear today")
	}
	svc.RestoreTask(removed, idx)

	// Subtask auto-completes parent task and unchecks on revert
	sub := svc.AddSubtask("today1", "第一步")
	if sub == nil {
		t.Fatal("add subtask failed")
	}
	justDone, allDone, _ := svc.ToggleSubtask("today1", sub.ID)
	if !justDone || !allDone {
		t.Fatal("completing last subtask should complete parent task")
	}
	_, allDone2, _ := svc.ToggleSubtask("today1", sub.ID)
	if allDone2 {
		t.Fatal("unchecking subtask should revert parent task")
	}
}
