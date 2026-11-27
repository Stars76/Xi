package timer

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Stars76/526Hz/internal/store"
)

type fakeClock struct {
	wall time.Time
	mono time.Duration
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{wall: start, mono: 0}
}

func (c *fakeClock) now() time.Time        { return c.wall }
func (c *fakeClock) m() time.Duration      { return c.mono }
func (c *fakeClock) advance(d time.Duration) { c.wall = c.wall.Add(d); c.mono += d }
func (c *fakeClock) jumpWall(d time.Duration) { c.wall = c.wall.Add(d) }

func newTestEnv(t *testing.T, clk *fakeClock) (*store.Store, *Engine, *[]Event, *[][2]string) {
	t.Helper()
	dir := t.TempDir()
	st := store.NewStore(dir, store.NewLogger(filepath.Join(dir, "logs")))
	// Cancel the debounced disk flush before TempDir cleanup runs, otherwise a late write races the removal.
	t.Cleanup(func() { _ = st.Flush() })
	st.Load()
	var events []Event
	var notifs [][2]string
	eng := NewEngine(Options{
		Store: st,
		Now:   clk.now,
		Mono:  clk.m,
		OnEvent: func(ev Event) {
			events = append(events, ev)
		},
		Notify: func(title, body string) {
			notifs = append(notifs, [2]string{title, body})
		},
	})
	eng.Init()
	return st, eng, &events, &notifs
}

func TestStateTransitionsAndSingleCompletion(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 9, 9, 10, 0, 0, 0, time.Local))
	_, eng, events, notifs := newTestEnv(t, clk)

	// Idle pause & abandon rejected
	s := eng.Command("pause")
	if s.SMState != "idle" || s.LastRejection == nil || s.LastRejection.Command != "pause" {
		t.Fatalf("expected pause rejection in idle, got %+v", s)
	}
	s = eng.Command("abandon")
	if s.LastRejection == nil || s.LastRejection.Command != "abandon" || len(eng.History()) != 0 {
		t.Fatalf("expected abandon rejection without history")
	}

	// Start -> pause 30s -> resume -> finish once
	eng.Command("start")
	clk.advance(10 * time.Minute)
	eng.Command("pause")
	clk.advance(30 * time.Second)
	eng.Command("start")
	clk.advance(15 * time.Minute)
	eng.Tick()
	eng.Command("refresh")

	s = eng.Snapshot()
	if s.Stats.Rounds != 1 || s.Stats.Minutes != 25 {
		t.Fatalf("expected 1 round / 25 min, got %+v", s.Stats)
	}
	doneCount := 0
	for _, ev := range *events {
		if ev.Type == "done" {
			doneCount++
			if ev.PausedMs < 29000 || ev.PausedMs > 31000 {
				t.Fatalf("expected ~30s pausedMs, got %d", ev.PausedMs)
			}
		}
	}
	if doneCount != 1 || len(*notifs) != 1 || len(eng.History()) != 1 {
		t.Fatalf("completion must happen exactly once: done=%d notif=%d hist=%d", doneCount, len(*notifs), len(eng.History()))
	}
}

func TestClockJumpMidnightAndRestartRecovery(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 9, 9, 23, 50, 0, 0, time.Local))
	st, eng, _, notifs := newTestEnv(t, clk)

	eng.Command("start")
	// System clock jumps back 2 hours; monotonic keeps real progress
	clk.jumpWall(-2 * time.Hour)
	eng.Tick()
	if eng.Snapshot().ClockJumpAt == 0 {
		t.Fatal("expected clock jump detection")
	}
	if eng.Snapshot().Timer.LeftSec < 24*60 {
		t.Fatalf("leftSec should be preserved around 25m, got %d", eng.Snapshot().Timer.LeftSec)
	}

	// Reset wall clock forward across midnight and finish
	clk.jumpWall(2 * time.Hour)
	clk.advance(25 * time.Minute)
	eng.Tick()
	if eng.Snapshot().Stats.Day != "2026-09-10" || eng.Snapshot().Stats.Rounds != 1 {
		t.Fatalf("cross-midnight stats should belong to completion day 2026-09-10, got %+v", eng.Snapshot().Stats)
	}

	// Restart recovery: overdue round silent completion
	eng.Command("start")
	eng.Flush()
	clk.advance(30 * time.Minute)

	st2 := store.NewStore(st.Dir(), nil)
	t.Cleanup(func() { _ = st2.Flush() })
	st2.Load()
	var events2 []Event
	notifsBefore := len(*notifs)
	eng2 := NewEngine(Options{
		Store: st2,
		Now:   clk.now,
		Mono:  clk.m,
		OnEvent: func(ev Event) {
			events2 = append(events2, ev)
		},
		Notify: func(title, body string) {
			*notifs = append(*notifs, [2]string{title, body})
		},
	})
	eng2.Init()
	if len(*notifs) != notifsBefore {
		t.Fatal("silent overdue recovery must not notify")
	}
	hist := eng2.History()
	if len(hist) != 2 || hist[1].Result != "recovered" {
		t.Fatalf("expected recovered history entry, got %+v", hist)
	}
}

func TestPomodoroPlanAndTaskBinding(t *testing.T) {
	clk := newFakeClock(time.Date(2026, 9, 9, 9, 0, 0, 0, time.Local))
	st, eng, events, _ := newTestEnv(t, clk)

	st.SetJSON(store.KeyStateV2, store.FocusState{
		ViewDay: "2026-09-09",
		Days: map[string]*store.DayBucket{
			"2026-09-09": {
				Tasks: []store.Task{
					{ID: "t1", Title: "写核心逻辑", Subtasks: []store.Subtask{}},
					{ID: "t2", Title: "其他任务", Subtasks: []store.Subtask{}},
				},
				DismissedDaily: map[string]bool{},
			},
		},
	})

	// Bind with noStart, then applyPlan keeps binding
	r1 := eng.StartTask("t1", "2026-09-09", true)
	if !r1.OK || !r1.Snapshot.Timer.AwaitPlan || r1.Snapshot.Timer.Task == nil || r1.Snapshot.Timer.Task.ID != "t1" {
		t.Fatalf("expected task bound awaiting plan, got %+v", r1)
	}
	eng.Command("applyPlan", CmdPayload{Plan: "sprint"})
	s := eng.Snapshot()
	if s.Timer.Plan == nil || s.Timer.Plan.Key != "sprint" || s.Timer.Task == nil || s.Timer.Task.ID != "t1" {
		t.Fatalf("applyPlan must preserve bound task, got %+v", s.Timer)
	}
	eng.Command("start")
	// Busy check against another task
	busy := eng.StartTask("t2", "2026-09-09", false)
	if busy.OK || busy.Code != "timer-busy" {
		t.Fatalf("expected timer-busy for second task, got %+v", busy)
	}

	// Finish first 15m focus segment -> advances to short break with plan-advance
	clk.advance(15 * time.Minute)
	eng.Tick()
	s = eng.Snapshot()
	if !s.Timer.Running || s.Timer.Phase != "short" || s.Timer.Plan == nil || s.Timer.Plan.Index != 1 {
		t.Fatalf("expected plan auto-advanced to short break step 1, got %+v", s.Timer)
	}
	advCount := 0
	for _, ev := range *events {
		if ev.Type == "plan-advance" && ev.PrevPhase == "focus" {
			advCount++
		}
	}
	if advCount != 1 {
		t.Fatalf("expected 1 plan-advance event with prevPhase=focus, got %d", advCount)
	}
}
