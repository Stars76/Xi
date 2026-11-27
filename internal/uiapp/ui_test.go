package uiapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Stars76/526Hz/internal/store"
	"github.com/Stars76/526Hz/internal/timer"
	"github.com/egoist/mygo/ui"
)

func newUITestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	now := time.Date(2026, 9, 9, 10, 0, 0, 0, time.Local)
	return NewApp(AppConfig{
		UserDataDir: dir,
		Now:         func() time.Time { return now },
	})
}

func TestMainViewTodoAndPanels(t *testing.T) {
	a := newUITestApp(t)
	tt := ui.NewTester(a.MainView, 1060, 780)

	if !tt.HasText("今日") {
		t.Fatalf("expected 今日事 title in main view, got %v", tt.Texts())
	}

	// Add a new task via TextInput + Submit
	if err := tt.Click("新任务"); err != nil {
		t.Fatal(err)
	}
	tt.Type("测试原生UI")
	tt.Key(0, ui.KeyEnter)
	if len(a.Todo.Tasks()) != 1 || a.Todo.Tasks()[0].Title != "测试原生UI" {
		t.Fatalf("expected new task created, got %+v", a.Todo.Tasks())
	}
	if !tt.HasText("测试原生UI") {
		t.Fatalf("expected new task rendered in list, got %v", tt.Texts())
	}

	// Switch to Calendar overlay and back
	if err := tt.Click("日历"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("返回任务") {
		t.Fatal("expected calendar overlay with 返回任务")
	}
	if err := tt.Click("返回任务"); err != nil {
		t.Fatal(err)
	}

	// Switch to Stats overlay and test Day / Week tabs
	if err := tt.Click("统计"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("专注统计") {
		t.Fatal("expected stats panel")
	}
	if err := tt.Click("每日"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("+ 补录时段") {
		t.Fatal("expected day timeline toolbar")
	}
	if err := tt.Click("返回统计"); err != nil {
		t.Fatal(err)
	}

	// Switch to Settings overlay and toggle dark theme
	if err := tt.Click("设置与备份"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("外观主题") {
		t.Fatal("expected settings panel")
	}
	if err := tt.Click("深色"); err != nil {
		t.Fatal(err)
	}
	if a.Timer.Prefs().Theme != "dark" {
		t.Fatalf("expected theme=dark after click, got %q", a.Timer.Prefs().Theme)
	}
}

func TestTimerViewPlanAndControls(t *testing.T) {
	a := newUITestApp(t)
	tt := ui.NewTester(a.TimerView, 320, 430)

	if !tt.HasText("25:00") {
		t.Fatalf("expected 25:00 initial countdown, got %v", tt.Texts())
	}

	// Open plan selector, pick 深专注 (45 min)
	if err := tt.Click("25 分钟"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("选择专注计划") {
		t.Fatal("expected plan selector overlay")
	}
	if err := tt.Click("深专注"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("45:00") || a.Timer.Snapshot().Timer.Plan == nil || a.Timer.Snapshot().Timer.Plan.Key != "deep" {
		t.Fatalf("expected deep plan 45:00 active, snap=%+v", a.Timer.Snapshot().Timer)
	}

	// Start -> Pause -> Reset
	if err := tt.Click("开始"); err != nil {
		t.Fatal(err)
	}
	if !a.Timer.Snapshot().Timer.Running || !tt.HasText("暂停") {
		t.Fatal("expected timer running and 暂停 button visible")
	}
	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	if a.Timer.Snapshot().Timer.Running || !tt.HasText("继续") {
		t.Fatal("expected timer paused and 继续 button visible")
	}
	// Resetting an in-progress round needs a confirming second click (it records an abandoned round).
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	if a.Timer.Snapshot().Timer.Idle || len(a.Timer.History()) != 0 || !tt.HasText("确认放弃") {
		t.Fatalf("expected first reset click to only arm confirmation, hist=%+v", a.Timer.History())
	}
	if err := tt.Click("确认放弃"); err != nil {
		t.Fatal(err)
	}
	if !a.Timer.Snapshot().Timer.Idle || len(a.Timer.History()) != 1 || a.Timer.History()[0].Result != "abandoned" {
		t.Fatalf("expected reset after start to record abandoned history, hist=%+v", a.Timer.History())
	}

	// Done screen rendering check
	a.timerDoneEv = &timer.Event{
		Type:           "done",
		Phase:          "focus",
		PlannedMinutes: 25,
		ActualMinutes:  25,
		Round:          1,
		MinutesToday:   25,
	}
	a.timerDoneVisible = true
	tt.Frame()
	if !tt.HasText("专注完成！") || !tt.HasText("再来一轮") {
		t.Fatalf("expected done screen rendered, got %v", tt.Texts())
	}
	_ = store.SchemaVersion
}

func TestElectronV2StoreRoundtripInUI(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 9, 14, 0, 0, 0, time.Local)
	v2JSON := `{
  "schema": 2,
  "updatedAt": "2026-09-09T06:00:00.000Z",
  "data": {
    "sisy-timer-prefs": {
      "sound": true,
      "notify": true,
      "motion": "full",
      "theme": "dark",
      "showSeconds": true,
      "phases": { "focus": 2700, "short": 300, "long": 900 }
    },
    "sisy-timer-stats": {
      "day": "2026-09-09",
      "rounds": 1,
      "minutes": 45
    },
    "sisy-timer-history": [
      {
        "day": "2026-09-09",
        "phase": "focus",
        "startedAt": "2026-09-09T01:15:00Z",
        "finishedAt": "2026-09-09T02:00:00Z",
        "plannedMinutes": 45,
        "actualMinutes": 45,
        "pauseCount": 0,
        "pausedMinutes": 0,
        "result": "done",
        "taskId": "v2_t1",
        "taskDay": "2026-09-09",
        "taskTitle": "兼容 Electron 旧版数据"
      }
    ],
    "sisy-focus-state-v2": {
      "viewDay": "2026-09-09",
      "days": {
        "2026-09-09": {
          "tasks": [
            {
              "id": "v2_t1",
              "title": "兼容 Electron 旧版数据",
              "done": false,
              "alarm": "15:30",
              "subtasks": [
                { "id": "v2_s1", "title": "核对子任务结构", "minutes": 15, "done": true }
              ]
            }
          ],
          "dismissedDaily": {}
        }
      }
    },
    "sisy-daily-config": {
      "tasks": [
        { "id": "v2_d1", "title": "每日复盘模板" }
      ]
    },
    "sisy-export-template": {
      "daily": "# {{date_cn}}\n{{tasks_done}}"
    }
  }
}`
	if err := os.WriteFile(filepath.Join(dir, "sisy-store.json"), []byte(v2JSON), 0o600); err != nil {
		t.Fatal(err)
	}

	a := NewApp(AppConfig{
		UserDataDir: dir,
		Now:         func() time.Time { return now },
	})
	if a.Timer.Prefs().Theme != "dark" {
		t.Fatalf("expected v2 dark theme preserved, got %q", a.Timer.Prefs().Theme)
	}
	var tplMap map[string]string
	if !a.Store.GetJSON(store.KeyExportTemplate, &tplMap) || tplMap["daily"] != "# {{date_cn}}\n{{tasks_done}}" {
		t.Fatalf("expected v2 export template preserved, got %+v", tplMap)
	}

	mainTT := ui.NewTester(a.MainView, 1060, 780)
	if !mainTT.HasText("兼容 Electron 旧版数据") {
		t.Fatalf("expected Electron v2 task rendered in MainView, got %v", mainTT.Texts())
	}

	timerTT := ui.NewTester(a.TimerView, 320, 430)
	if !timerTT.HasText("45:00") {
		t.Fatalf("expected 45:00 countdown from v2 prefs, got %v", timerTT.Texts())
	}

	if err := a.Store.Flush(); err != nil {
		t.Fatal(err)
	}
	rawOut, err := os.ReadFile(filepath.Join(dir, "sisy-store.json"))
	if err != nil {
		t.Fatal(err)
	}
	var env store.Envelope
	if err := json.Unmarshal(rawOut, &env); err != nil {
		t.Fatal(err)
	}
	if env.Schema != store.SchemaVersion {
		t.Fatalf("expected schema=%d on disk, got %d", store.SchemaVersion, env.Schema)
	}
}

func TestSmartAlarmNLPAndMiniPill(t *testing.T) {
	a := newUITestApp(t)
	tt := ui.NewTester(a.MainView, 1140, 780)

	// Confirm removed Ctrl+K search bar is gone
	if tt.HasText("搜索或快捷操作…") {
		t.Fatal("expected Ctrl+K command bar completely removed from titlebar")
	}

	// 1) Smart natural-language alarm parsing via task input
	if err := tt.Click("新任务"); err != nil {
		t.Fatal(err)
	}
	tt.Type("整理周报 16:30")
	tt.Key(0, ui.KeyEnter)
	tasks := a.Todo.Tasks()
	if len(tasks) != 1 || tasks[0].Title != "整理周报" || tasks[0].Alarm != "16:30" {
		t.Fatalf("expected NLP alarm parsed (title=整理周报, alarm=16:30), got %+v", tasks)
	}

	// 2) Toggle Focus Timer Mini Pill mode and expand back
	tTT := ui.NewTester(a.TimerView, 320, 430)
	if err := tTT.Click("切换灵动条"); err != nil {
		t.Fatal(err)
	}
	if !a.timerMiniPill {
		t.Fatal("expected timerMiniPill=true after click")
	}
	tTT.SetSize(264, 50)
	tTT.Frame()
	if !tTT.HasText("25:00") {
		t.Fatalf("expected 25:00 visible in mini pill, got %v", tTT.Texts())
	}
	if _, ok := tTT.Find("关闭灵动条"); !ok {
		t.Fatal("expected direct close button on mini pill")
	}
	if err := tTT.Click("展开专注钟"); err != nil {
		t.Fatal(err)
	}
	if a.timerMiniPill {
		t.Fatal("expected timerMiniPill=false after expand click")
	}
}

func TestUnifiedFocusStageAndSubtaskSync(t *testing.T) {
	a := newUITestApp(t)
	tk := a.Todo.AddTask("融合字符海与任务")
	if tk == nil {
		t.Fatal("failed to seed task")
	}
	sub := a.Todo.AddSubtask(tk.ID, "在专注舞台直接勾选步骤")
	if sub == nil {
		t.Fatal("failed to seed subtask")
	}
	a.Timer.StartTask(tk.ID, a.Todo.TodayKey(), false)

	tt := ui.NewTester(a.MainView, 1120, 780)
	if !tt.HasText("今日专注") || !tt.HasText("汐") {
		t.Fatalf("expected 汐 brand and sidebar focus card, got %v", tt.Texts())
	}

	// Enter unified Focus Clock inside main window
	if err := tt.Click("专注钟"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("当前任务") || !tt.HasText("在专注舞台直接勾选步骤") {
		t.Fatalf("expected bound task and subtask rendered inside Focus Clock, got %v", tt.Texts())
	}

	// Check off subtask directly inside Focus Stage
	if err := tt.Click("舞台步骤 在专注舞台直接勾选步骤"); err != nil {
		t.Fatal(err)
	}
	if !a.Todo.Tasks()[0].Subtasks[0].Done {
		t.Fatal("expected subtask marked done from inside Focus Stage")
	}

	// Pause and resume timer directly from Focus Stage (StartTask already auto-started)
	if !a.Timer.Snapshot().Timer.Running {
		t.Fatal("expected timer running after StartTask")
	}
	if err := tt.Click("暂停"); err != nil {
		t.Fatal(err)
	}
	if a.Timer.Snapshot().Timer.Running {
		t.Fatal("expected timer paused after Focus Stage Pause")
	}
	if err := tt.Click("继续"); err != nil {
		t.Fatal(err)
	}
	if !a.Timer.Snapshot().Timer.Running {
		t.Fatal("expected timer running after Focus Stage Resume")
	}
}

func TestDeleteUnboundFocusRecordInStats(t *testing.T) {
	a := newUITestApp(t)
	a.Store.SetJSON(store.KeyTimerStats, store.TimerStats{Day: "2026-09-09", Rounds: 2, Minutes: 50})
	a.Store.SetJSON(store.KeyTimerHistory, []store.HistoryEntry{
		{Day: "2026-09-09", Phase: "focus", StartedAt: "2026-09-09T01:00:00Z", FinishedAt: "2026-09-09T01:25:00Z", PlannedMinutes: 25, ActualMinutes: 25, Result: "done"},
		{Day: "2026-09-09", Phase: "focus", StartedAt: "2026-09-09T02:00:00Z", FinishedAt: "2026-09-09T02:25:00Z", PlannedMinutes: 25, ActualMinutes: 25, Result: "done", TaskID: "t1", TaskDay: "2026-09-09", TaskTitle: "正常任务"},
	})
	a.Timer.ReloadFromStore()
	a.activeOverlay = "stats"
	a.statsTab = 1 // 本周

	tt := ui.NewTester(a.MainView, 1140, 820)
	if !tt.HasText("未定任务的专注") {
		t.Fatalf("expected unbound focus session listed in stats, got %v", tt.Texts())
	}
	if err := tt.Click("筛选未定任务"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("正常任务") || !tt.HasText("未定任务的专注") {
		t.Fatalf("expected only unbound sessions after filter, got %v", tt.Texts())
	}
	if err := tt.Click("删除专注 未定任务的专注 0"); err != nil {
		t.Fatal(err)
	}
	if len(a.Timer.History()) != 1 || a.Timer.Snapshot().Stats.Minutes != 25 || a.Timer.Snapshot().Stats.Rounds != 1 {
		t.Fatalf("expected history & today stats updated after delete, hist=%+v stats=%+v", a.Timer.History(), a.Timer.Snapshot().Stats)
	}
}

func TestAuditUXFixes(t *testing.T) {
	a := newUITestApp(t)
	tk := a.Todo.AddTask("写季度复盘")
	sub := a.Todo.AddSubtask(tk.ID, "核对核心数据")
	if tk == nil || sub == nil {
		t.Fatal("failed to seed task/subtask")
	}

	tt := ui.NewTester(a.MainView, 1120, 780)
	if err := tt.Click("新任务"); err != nil {
		t.Fatal(err)
	}
	tt.Type("read pdf file")
	tt.Key(0, ui.KeyEnter)
	if a.activeOverlay != "" {
		t.Fatalf("typing 'f' in 新任务 must not launch Focus Clock, got overlay=%q", a.activeOverlay)
	}
	if len(a.Todo.Tasks()) != 2 || a.Todo.Tasks()[0].Title != "read pdf file" {
		t.Fatalf("expected new task added cleanly, got %+v", a.Todo.Tasks())
	}

	// Bind today's task, switch Todo to yesterday, then open Focus Clock -> bound task must STILL show and subtask toggle must update today's bucket!
	today := a.Todo.TodayKey()
	a.Timer.StartTask(tk.ID, today, false)
	a.Todo.ShiftView(-1)
	a.activeOverlay = "focus"
	tt.Frame()
	if !tt.HasText("当前任务") || !tt.HasText("核对核心数据") {
		t.Fatalf("expected bound task visible in Focus Clock even when Todo views past day, got %v", tt.Texts())
	}
	if err := tt.Click("舞台步骤 核对核心数据"); err != nil {
		t.Fatal(err)
	}
	todayTasks := a.Todo.TasksForDay(today)
	foundDone := false
	for _, x := range todayTasks {
		if x.ID == tk.ID && len(x.Subtasks) > 0 && x.Subtasks[0].Done {
			foundDone = true
		}
	}
	if !foundDone {
		t.Fatal("expected today's subtask toggled done from Focus Clock while Todo views yesterday")
	}

	// Switching custom/single focus duration preserves bound task
	a.Timer.Command("setDuration", timer.CmdPayload{Sec: 45 * 60})
	snap := a.Timer.Snapshot()
	if snap.Timer.Task == nil || snap.Timer.Task.ID != tk.ID {
		t.Fatalf("expected bound task preserved after setDuration, got %+v", snap.Timer.Task)
	}
}

func TestSeaScenePresetsEncountersAndYearlyHeatmap(t *testing.T) {
	a := newUITestApp(t)
	a.activeOverlay = "focus"
	tt := ui.NewTester(a.MainView, 1140, 820)

	if err := tt.Click("海域场景"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("场景预设") || !tt.HasText("环境音") || !tt.HasText("蓝鲸") || !tt.HasText("海豚") || !tt.HasText("极光") {
		t.Fatalf("expected presets, ambient noise, and 6-card atlas visible, got %v", tt.Texts())
	}

	// Apply built-in preset 雨夜 -> dark theme + rain + ambient on + 45m
	if err := tt.Click("预设 雨夜"); err != nil {
		t.Fatal(err)
	}
	if a.Timer.Prefs().Theme != "dark" || a.effectiveWeather() != "rain" || !a.seaSceneSnapshot().AmbientOn || a.Timer.Snapshot().Timer.TotalSec != 45*60 {
		t.Fatalf("preset 雨夜 did not apply cleanly: theme=%s weather=%s ambient=%v sec=%d",
			a.Timer.Prefs().Theme, a.effectiveWeather(), a.seaSceneSnapshot().AmbientOn, a.Timer.Snapshot().Timer.TotalSec)
	}

	// Save current as custom preset
	if err := tt.Click("保存当前为我的预设"); err != nil {
		t.Fatal(err)
	}
	if a.seaSceneSnapshot().CustomPreset == nil {
		t.Fatal("expected custom preset saved")
	}

	// Summon Angler Boat, render frame to record bounds, then click on it to catch a fish!
	if err := tt.Click("召唤 垂钓小船"); err != nil {
		t.Fatal(err)
	}
	if a.sceneModalOpen {
		t.Fatal("expected scene modal closed after summon")
	}
	a.stageSea.TriggerEncounter(EncounterAngler, a.now().Add(-10*time.Second), false)
	tt.Frame()
	a.stageSea.mu.Lock()
	b := a.stageSea.scene.active.bounds
	a.stageSea.mu.Unlock()
	if b.W <= 0 {
		t.Fatal("expected encounter bounds recorded during draw")
	}
	a.handleOceanClick(b.X+b.W/2, b.Y+b.H/2)
	if a.seaSceneSnapshot().FishCaught != 1 {
		t.Fatalf("expected clicking angler boat to increment FishCaught=1, got %d", a.seaSceneSnapshot().FishCaught)
	}

	// Record natural sighting and verify 6-item Atlas counter
	a.recordNaturalEncounter(EncounterDolphin)
	a.sceneModalOpen = true
	tt.Frame()
	if !tt.HasText("1 / 6") {
		t.Fatalf("expected atlas updated with 1 / 6, got %v", tt.Texts())
	}
	a.sceneModalOpen = false

	// Switch to Stats -> 年度 tab and verify 52-week Heatmap renders cleanly
	a.activeOverlay = "stats"
	a.statsTab = 3
	tt.Frame()
	if !tt.HasText("年度热力图") || !tt.HasText("年度专注分钟") {
		t.Fatalf("expected yearly heatmap tab rendered, got %v", tt.Texts())
	}
}

func TestDoneGroupFoldAndSharedPiPOcean(t *testing.T) {
	a := newUITestApp(t)
	t1 := a.Todo.AddTask("进行中任务")
	t2 := a.Todo.AddTask("已完成示例任务")
	if t1 == nil || t2 == nil {
		t.Fatal("failed to create test tasks")
	}
	a.Todo.ToggleTask(t2.ID)

	tt := ui.NewTester(a.MainView, 1060, 760)
	// Idle on Today page: redundant titlebar chip is hidden; completed task is folded under 已完成
	if tt.HasText("已完成示例任务") {
		t.Fatalf("expected completed task folded by default, got %v", tt.Texts())
	}
	if !tt.HasText("已完成") {
		t.Fatalf("expected collapsible 已完成 group header, got %v", tt.Texts())
	}

	// Expand completed group -> completed task becomes visible
	if err := tt.Click("已完成 1"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("已完成示例任务") {
		t.Fatalf("expected completed task visible after expanding group, got %v", tt.Texts())
	}

	// Trigger encounter on shared stageSea and render PiP TimerView -> shares same ocean bounds
	a.stageSea.TriggerEncounter(EncounterWhale, a.now().Add(-5*time.Second), false)
	pip := ui.NewTester(a.TimerView, 320, 430)
	pip.Frame()
	if a.stageSea.ActiveEncounterKind(a.now()) != EncounterWhale {
		t.Fatal("expected PiP TimerView to share stageSea encounter")
	}
}

func TestP0FlowFixes(t *testing.T) {
	a := newUITestApp(t)
	tt := ui.NewTester(a.MainView, 1060, 780)
	today := a.Todo.TodayKey()

	// Planning ahead: tomorrow is reachable from Today and accepts new tasks.
	if err := tt.Click("后一天"); err != nil {
		t.Fatal(err)
	}
	tomorrow := a.Todo.ViewDay()
	if tomorrow <= today || !tt.HasText("明日") {
		t.Fatalf("expected to view tomorrow, viewDay=%s today=%s texts=%v", tomorrow, today, tt.Texts())
	}
	if err := tt.Click("新任务"); err != nil {
		t.Fatal(err)
	}
	tt.Type("明天的计划")
	tt.Key(0, ui.KeyEnter)
	if got := a.Todo.TasksForDay(tomorrow); len(got) != 1 || got[0].Title != "明天的计划" {
		t.Fatalf("expected task planned for tomorrow, got %+v", got)
	}
	a.Todo.Init() // reload from disk + rollover must leave future tasks in place
	if got := a.Todo.TasksForDay(tomorrow); len(got) != 1 {
		t.Fatalf("expected future task to survive reload/rollover, got %+v", got)
	}
	if len(a.Todo.TasksForDay(today)) != 0 {
		t.Fatalf("future task must not leak into today: %+v", a.Todo.TasksForDay(today))
	}
	a.Todo.GoToday()

	// Starting from Today with open tasks and nothing bound asks which task the round is for.
	task := a.Todo.AddTask("写周报")
	if task == nil {
		t.Fatal("add task failed")
	}
	taskID := task.ID
	tt.Frame()
	if err := tt.Click("开始专注"); err != nil {
		t.Fatal(err)
	}
	if a.Timer.Snapshot().Timer.Running || !tt.HasText("这一轮专注做哪件事？") {
		t.Fatalf("expected bind picker instead of an unbound start, texts=%v", tt.Texts())
	}
	if err := tt.Click("专注 写周报"); err != nil {
		t.Fatal(err)
	}
	snap := a.Timer.Snapshot()
	if !snap.Timer.Running || snap.Timer.Task == nil || snap.Timer.Task.ID != taskID {
		t.Fatalf("expected round running and bound to picked task, got %+v", snap.Timer)
	}

	// A round finishing while on Today (not the Focus Clock page) still surfaces the completion card.
	a.timerDoneEv = &timer.Event{Type: "done", Phase: "focus", PlannedMinutes: 25, Round: 1, TaskID: taskID, TaskTitle: "写周报", Day: today}
	a.timerDoneVisible = true
	tt.Frame()
	if !tt.HasText("专注完成！") {
		t.Fatalf("expected completion card on Today page, texts=%v", tt.Texts())
	}
	if err := tt.Click("知道了"); err != nil {
		t.Fatal(err)
	}
	if a.timerDoneVisible {
		t.Fatal("expected completion card dismissed")
	}

	// Reset and verify the task-row focus button starts in one click (no plan-picker detour).
	a.Timer.Command("reset")
	a.Timer.Command("reset")
	if !a.Timer.Snapshot().Timer.Idle {
		t.Fatal("expected idle timer after reset")
	}
	tt.Frame()
	r, ok := tt.Find("写周报")
	if !ok {
		t.Fatalf("task row not rendered, texts=%v", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2) // row actions are revealed on hover
	if err := tt.Click("开始专注 写周报"); err != nil {
		t.Fatal(err)
	}
	snap = a.Timer.Snapshot()
	if !snap.Timer.Running || snap.Timer.Task == nil || snap.Timer.Task.ID != taskID || a.timerShowSetup {
		t.Fatalf("expected one-click start bound to task without setup overlay, timer=%+v setup=%v", snap.Timer, a.timerShowSetup)
	}
	if a.activeOverlay != "focus" {
		t.Fatalf("expected to land on Focus Clock, overlay=%q", a.activeOverlay)
	}
}

func TestP1KeyboardMenuAndPersistedLayout(t *testing.T) {
	a := newUITestApp(t)
	tt := ui.NewTester(a.MainView, 1060, 780)

	// Ctrl+1..5 switch pages in sidebar order.
	for i, want := range []string{"", "focus", "calendar", "stats", "settings"} {
		tt.Key(ui.Ctrl, ui.Key1+ui.Key(i))
		if a.activeOverlay != want {
			t.Fatalf("Ctrl+%d: expected page %q, got %q", i+1, want, a.activeOverlay)
		}
	}
	// The Settings page now carries the focus-clock preferences and the shortcut reference.
	if !tt.HasText("专注与提醒") || !tt.HasText("提示音") || !tt.HasText("快捷键") {
		t.Fatalf("expected timer prefs + shortcuts on settings page, texts=%v", tt.Texts())
	}

	// Ctrl+N returns to Today with the new-task input focused: typing goes straight into it.
	tt.Key(ui.Ctrl, ui.KeyN)
	if a.activeOverlay != "" {
		t.Fatalf("Ctrl+N should land on Today, got %q", a.activeOverlay)
	}
	tt.Type("键盘 建的 任务")
	tt.Key(0, ui.KeySpace) // Space inside a text input must not start the timer
	if a.Timer.Snapshot().Timer.Running || a.bindPickOpen {
		t.Fatal("Space typed into a text input must not trigger the timer shortcut")
	}
	tt.Key(0, ui.KeyEnter)
	tasks := a.Todo.Tasks()
	if len(tasks) != 1 || tasks[0].Title != "键盘 建的 任务" {
		t.Fatalf("expected task created from keyboard, got %+v", tasks)
	}

	// Right-click menu exposes the row actions without hovering.
	if err := tt.RightClick("键盘 建的 任务"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("添加小步骤") || !tt.HasText("删除任务") {
		t.Fatalf("expected task context menu, texts=%v", tt.Texts())
	}
	if err := tt.Click("修改标题"); err != nil {
		t.Fatal(err)
	}
	if a.editingTaskID != tasks[0].ID {
		t.Fatal("expected context menu 修改标题 to start inline edit")
	}
	tt.Key(0, ui.KeyEscape)

	// Layout toggles persist across restarts.
	if err := tt.Click("切换侧栏"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("切换今日时间轴侧栏"); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.Flush(); err != nil {
		t.Fatal(err)
	}
	b := NewApp(AppConfig{UserDataDir: a.Store.Dir(), Now: a.nowFn})
	if !b.sidebarCollapsed || !b.tlDrawerOpen {
		t.Fatalf("expected layout restored, sidebarCollapsed=%v tlDrawerOpen=%v", b.sidebarCollapsed, b.tlDrawerOpen)
	}
}

func TestP1SpaceShortcutAndFullBindStrip(t *testing.T) {
	a := newUITestApp(t)
	tt := ui.NewTester(a.MainView, 1060, 780)
	for _, title := range []string{"任务一", "任务二", "任务三", "任务四", "任务五"} {
		a.Todo.AddTask(title)
	}
	tt.Frame()

	// Space on Today with nothing bound opens the task picker instead of an unbound start.
	tt.Key(0, ui.KeySpace)
	if a.Timer.Snapshot().Timer.Running || !a.bindPickOpen {
		t.Fatal("expected Space on Today to open the bind picker")
	}
	tt.Key(0, ui.KeyEscape)

	// On the Focus Clock, the quick-bind strip can reach every open task, not just the first three.
	tt.Key(ui.Ctrl, ui.Key2)
	if tt.HasText("任务一") { // newest first: 任务五/四/三 are shown, 任务一 is hidden
		t.Fatalf("expected strip collapsed to three tasks, texts=%v", tt.Texts())
	}
	if err := tt.Click("展开全部今日任务"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("绑定 任务一"); err != nil {
		t.Fatal(err)
	}
	snap := a.Timer.Snapshot()
	if !snap.Timer.Running || snap.Timer.Task == nil || snap.Timer.Task.Title != "任务一" {
		t.Fatalf("expected round bound to the previously unreachable task, got %+v", snap.Timer)
	}

	// Space toggles pause / resume while a round is active.
	tt.Key(0, ui.KeySpace)
	if a.Timer.Snapshot().Timer.Running {
		t.Fatal("expected Space to pause the running round")
	}
	tt.Key(0, ui.KeySpace)
	if !a.Timer.Snapshot().Timer.Running {
		t.Fatal("expected Space to resume the paused round")
	}
}

func TestP2ResponsiveShellAndYearDetail(t *testing.T) {
	a := newUITestApp(t)
	today := a.Todo.TodayKey()
	// One finished 50-minute round today feeds the year heatmap, month bars and day detail.
	a.Timer.RestoreHistoryEntries([]store.HistoryEntry{{
		Day: today, Phase: "focus", Result: "done", PlannedMinutes: 50, ActualMinutes: 50,
		StartedAt:  a.now().Add(-time.Hour).UTC().Format(time.RFC3339Nano),
		FinishedAt: a.now().Add(-10 * time.Minute).UTC().Format(time.RFC3339Nano),
	}})
	a.markStatsDirty()
	tt := ui.NewTester(a.MainView, 1060, 780)

	// Wide window: full sidebar with the info-only summary card (pop-outs live on the Focus Clock).
	if !slices.Contains(tt.Texts(), "今日专注") || tt.HasText("画中画") {
		t.Fatalf("expected info-only sidebar summary on a wide window, texts=%v", tt.Texts())
	}

	// Year tab: month bars and the pinned-day detail fill the space under the heatmap.
	tt.Key(ui.Ctrl, ui.Key4)
	a.statsTab = 3
	tt.Frame()
	if !tt.HasText("月度专注") || !tt.HasText("当日详情") || !tt.HasText(today) {
		t.Fatalf("expected month bars + day detail on year tab, texts=%v", tt.Texts())
	}
	if !tt.HasText("最高 9月 · 50 分钟") {
		t.Fatalf("expected best-month summary from seeded history, texts=%v", tt.Texts())
	}
	if err := tt.Click("查看当日时间轴"); err != nil {
		t.Fatal(err)
	}
	if a.statsTab != 0 || a.tlDayKey != today {
		t.Fatalf("expected jump to the day timeline, tab=%d day=%s", a.statsTab, a.tlDayKey)
	}

	// Narrow window: the sidebar folds to an icon rail that still navigates, and the summary card goes away.
	tt.Key(ui.Ctrl, ui.Key1)
	tt.SetSize(720, 560)
	tt.Frame()
	if slices.Contains(tt.Texts(), "今日专注") { // exact match: the Today status line also mentions 今日专注
		t.Fatalf("expected sidebar summary hidden in rail mode, texts=%v", tt.Texts())
	}
	if err := tt.Click("专注钟"); err != nil {
		t.Fatal(err)
	}
	if a.activeOverlay != "focus" {
		t.Fatalf("expected rail icon to navigate, overlay=%q", a.activeOverlay)
	}
	// No room for the side drawer: the timeline button opens the full day timeline instead.
	tt.Key(ui.Ctrl, ui.Key1)
	if err := tt.Click("切换今日时间轴侧栏"); err != nil {
		t.Fatal(err)
	}
	if a.tlDrawerOpen || a.activeOverlay != "stats" || a.statsTab != 0 {
		t.Fatalf("expected narrow timeline button to open stats day tab, drawer=%v overlay=%q", a.tlDrawerOpen, a.activeOverlay)
	}
}
