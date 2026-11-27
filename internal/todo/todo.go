package todo

import (
	"slices"
	"strings"
	"time"

	"github.com/Stars76/Xi/internal/dateutil"
	"github.com/Stars76/Xi/internal/store"
)

// Service manages tasks, subtasks, daily templates, and task rollover in sisy-focus-state-v2.
type Service struct {
	st    *store.Store
	log   *store.Logger
	now   func() time.Time
	state store.FocusState
}

func NewService(st *store.Store, logger *store.Logger, nowFn ...func() time.Time) *Service {
	nf := time.Now
	if len(nowFn) > 0 && nowFn[0] != nil {
		nf = nowFn[0]
	}
	return &Service{st: st, log: logger, now: nf}
}

func (s *Service) Init() {
	s.Load()
	s.RolloverOpenTasks()
	s.SyncDaily()
}

func (s *Service) Load() {
	var raw store.FocusState
	days := map[string]*store.DayBucket{}
	viewDay := dateutil.TodayKey(s.now())
	if s.st.GetJSON(store.KeyStateV2, &raw) && raw.Days != nil {
		for k, b := range raw.Days {
			if dateutil.IsValidKey(k) && b != nil {
				if b.Tasks == nil {
					b.Tasks = []store.Task{}
				}
				if b.DismissedDaily == nil {
					b.DismissedDaily = map[string]bool{}
				}
				days[k] = b
			}
		}
		if dateutil.IsValidKey(raw.ViewDay) {
			viewDay = raw.ViewDay
		}
	}
	if days[viewDay] == nil {
		days[viewDay] = &store.DayBucket{Tasks: []store.Task{}, DismissedDaily: map[string]bool{}}
	}
	s.state = store.FocusState{ViewDay: viewDay, Days: days}
}

func (s *Service) persist() bool {
	res := s.st.SetJSON(store.KeyStateV2, s.state)
	if !res.OK && s.log != nil {
		s.log.Error("保存今日事失败", res.Error)
	}
	return res.OK
}

func (s *Service) day(key string) *store.DayBucket {
	b := s.state.Days[key]
	if b == nil {
		b = &store.DayBucket{Tasks: []store.Task{}, DismissedDaily: map[string]bool{}}
		s.state.Days[key] = b
	}
	return b
}

func (s *Service) TodayKey() string {
	return dateutil.TodayKey(s.now())
}

func (s *Service) ViewDay() string {
	return s.state.ViewDay
}

func (s *Service) IsToday() bool {
	return s.state.ViewDay == s.TodayKey()
}

func (s *Service) Tasks() []store.Task {
	return s.day(s.state.ViewDay).Tasks
}

// TasksForDay returns tasks for a specific YYYY-MM-DD day without changing ViewDay.
func (s *Service) TasksForDay(key string) []store.Task {
	nk := dateutil.NormalizeKey(key)
	if nk == "" {
		return nil
	}
	b := s.state.Days[nk]
	if b == nil {
		return nil
	}
	return b.Tasks
}

// ToggleTaskInDay toggles a task's completion status in a specific day bucket.
func (s *Service) ToggleTaskInDay(dayKey, id string) bool {
	nk := dateutil.NormalizeKey(dayKey)
	if nk == "" {
		nk = s.state.ViewDay
	}
	b := s.state.Days[nk]
	if b == nil {
		return false
	}
	for i := range b.Tasks {
		if b.Tasks[i].ID == id {
			b.Tasks[i].Done = !b.Tasks[i].Done
			s.persist()
			return true
		}
	}
	return false
}

// ToggleSubtaskInDay toggles a subtask's completion status in a specific day bucket.
func (s *Service) ToggleSubtaskInDay(dayKey, taskID, subID string) bool {
	nk := dateutil.NormalizeKey(dayKey)
	if nk == "" {
		nk = s.state.ViewDay
	}
	b := s.state.Days[nk]
	if b == nil {
		return false
	}
	for i := range b.Tasks {
		if b.Tasks[i].ID == taskID {
			for j := range b.Tasks[i].Subtasks {
				if b.Tasks[i].Subtasks[j].ID == subID {
					b.Tasks[i].Subtasks[j].Done = !b.Tasks[i].Subtasks[j].Done
					s.persist()
					return true
				}
			}
			return false
		}
	}
	return false
}

func (s *Service) HasTasks(key string) bool {
	b := s.state.Days[key]
	return b != nil && len(b.Tasks) > 0
}

func (s *Service) Days() map[string]*store.DayBucket {
	return s.state.Days
}

func (s *Service) SetViewDay(key string) {
	nk := dateutil.NormalizeKey(key)
	if nk == "" || nk == s.state.ViewDay {
		return
	}
	s.state.ViewDay = nk
	s.day(nk)
	s.persist()
}

func (s *Service) ShiftView(delta int) {
	s.SetViewDay(dateutil.ShiftDay(s.state.ViewDay, delta))
}

func (s *Service) GoToday() {
	s.SetViewDay(s.TodayKey())
	s.SyncDaily()
}

// RolloverOpenTasks carries unfinished non-daily tasks from earlier dates to today at top.
func (s *Service) RolloverOpenTasks() bool {
	today := s.TodayKey()
	keys := make([]string, 0, len(s.state.Days))
	for k := range s.state.Days {
		if k < today {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)

	moved := false
	for _, k := range keys {
		dd := s.state.Days[k]
		if dd == nil || len(dd.Tasks) == 0 {
			continue
		}
		var keep, goTasks []store.Task
		for _, t := range dd.Tasks {
			if !t.Done && t.DailyID == "" {
				if t.RolledFrom == "" {
					t.RolledFrom = k
				}
				goTasks = append(goTasks, t)
				moved = true
			} else {
				keep = append(keep, t)
			}
		}
		if len(goTasks) > 0 {
			dd.Tasks = keep
			todayBucket := s.day(today)
			todayBucket.Tasks = append(goTasks, todayBucket.Tasks...)
		}
	}
	if moved {
		s.persist()
	}
	return moved
}

func (s *Service) DailyConfig() store.DailyConfig {
	var cfg store.DailyConfig
	if s.st.GetJSON(store.KeyDaily, &cfg) && cfg.Tasks != nil {
		out := make([]store.DailyTemplate, 0, len(cfg.Tasks))
		for _, t := range cfg.Tasks {
			title := strings.TrimSpace(t.Title)
			if title == "" {
				continue
			}
			id := t.ID
			if id == "" {
				id = store.RandID("d")
			}
			out = append(out, store.DailyTemplate{ID: id, Title: title})
		}
		return store.DailyConfig{Tasks: out}
	}
	return store.DailyConfig{Tasks: []store.DailyTemplate{}}
}

func (s *Service) SyncDaily() bool {
	cfg := s.DailyConfig()
	if len(cfg.Tasks) == 0 {
		return false
	}
	dd := s.day(s.TodayKey())
	had := map[string]bool{}
	for _, t := range dd.Tasks {
		if t.DailyID != "" {
			had[t.DailyID] = true
		}
	}
	added := false
	for _, dt := range cfg.Tasks {
		if had[dt.ID] || dd.DismissedDaily[dt.ID] {
			continue
		}
		dd.Tasks = append(dd.Tasks, store.Task{
			ID:        store.RandID("t"),
			DailyID:   dt.ID,
			Title:     dt.Title,
			Subtasks:  []store.Subtask{},
			Done:      false,
			CreatedAt: s.now().UTC().Format(time.RFC3339Nano),
		})
		added = true
	}
	if added {
		s.persist()
	}
	return added
}

func (s *Service) findTask(id string) (*store.Task, int) {
	tasks := s.day(s.state.ViewDay).Tasks
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i], i
		}
	}
	return nil, -1
}

func (s *Service) AddTask(title string) *store.Task {
	v := strings.TrimSpace(title)
	if v == "" {
		return nil
	}
	t := store.Task{
		ID:        store.RandID("t"),
		Title:     v,
		Subtasks:  []store.Subtask{},
		Done:      false,
		CreatedAt: s.now().UTC().Format(time.RFC3339Nano),
	}
	dd := s.day(s.state.ViewDay)
	dd.Tasks = append([]store.Task{t}, dd.Tasks...)
	s.persist()
	return &dd.Tasks[0]
}

func (s *Service) SetTaskTitle(id, title string) bool {
	t, _ := s.findTask(id)
	v := strings.TrimSpace(title)
	if t == nil || v == "" {
		return false
	}
	t.Title = v
	s.persist()
	return true
}

func (s *Service) SetTaskAlarm(id, hhmm string) bool {
	t, _ := s.findTask(id)
	if t == nil {
		return false
	}
	v := strings.TrimSpace(hhmm)
	if v == "" {
		if t.Alarm != "" {
			t.Alarm = ""
			s.persist()
		}
		return true
	}
	norm, ok := dateutil.NormalizeAlarm(v)
	if !ok {
		return false
	}
	t.Alarm = norm
	s.persist()
	return true
}

func (s *Service) ToggleTask(id string) (bool, bool) {
	t, _ := s.findTask(id)
	if t == nil {
		return false, false
	}
	t.Done = !t.Done
	s.persist()
	return t.Done, true
}

// DeleteTask deletes the task and returns a snapshot for undo.
func (s *Service) DeleteTask(id string) (store.Task, int, bool) {
	dd := s.day(s.state.ViewDay)
	t, idx := s.findTask(id)
	if t == nil {
		return store.Task{}, -1, false
	}
	removed := *t
	removed.Subtasks = slices.Clone(t.Subtasks)
	if t.DailyID != "" {
		dd.DismissedDaily[t.DailyID] = true
	}
	dd.Tasks = append(dd.Tasks[:idx], dd.Tasks[idx+1:]...)
	s.persist()
	return removed, idx, true
}

func (s *Service) RestoreTask(task store.Task, idx int) bool {
	dd := s.day(s.state.ViewDay)
	if task.DailyID != "" {
		delete(dd.DismissedDaily, task.DailyID)
	}
	if idx < 0 || idx > len(dd.Tasks) {
		idx = len(dd.Tasks)
	}
	dd.Tasks = slices.Insert(dd.Tasks, idx, task)
	s.persist()
	return true
}

func (s *Service) MoveTask(id, targetID string, after bool) bool {
	if !s.IsToday() || id == targetID {
		return false
	}
	dd := s.day(s.state.ViewDay)
	from := -1
	for i := range dd.Tasks {
		if dd.Tasks[i].ID == id {
			from = i
			break
		}
	}
	if from < 0 {
		return false
	}
	if targetID != "" {
		found := false
		for i := range dd.Tasks {
			if dd.Tasks[i].ID == targetID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	item := dd.Tasks[from]
	dd.Tasks = append(dd.Tasks[:from], dd.Tasks[from+1:]...)
	to := len(dd.Tasks)
	if targetID != "" {
		for i := range dd.Tasks {
			if dd.Tasks[i].ID == targetID {
				to = i
				if after {
					to++
				}
				break
			}
		}
	}
	dd.Tasks = slices.Insert(dd.Tasks, to, item)
	s.persist()
	return true
}

func (s *Service) MoveTaskIndex(from, to int) bool {
	if !s.IsToday() {
		return false
	}
	dd := s.day(s.state.ViewDay)
	if from < 0 || from >= len(dd.Tasks) || to < 0 || to > len(dd.Tasks) || from == to {
		return false
	}
	item := dd.Tasks[from]
	dd.Tasks = append(dd.Tasks[:from], dd.Tasks[from+1:]...)
	if to > from {
		to--
	}
	if to < 0 {
		to = 0
	}
	if to > len(dd.Tasks) {
		to = len(dd.Tasks)
	}
	dd.Tasks = slices.Insert(dd.Tasks, to, item)
	s.persist()
	return true
}

func (s *Service) AddSubtask(taskID, title string) *store.Subtask {
	t, _ := s.findTask(taskID)
	v := strings.TrimSpace(title)
	if t == nil || v == "" {
		return nil
	}
	sub := store.Subtask{ID: store.RandID("s"), Title: v, Minutes: 5, Done: false}
	t.Subtasks = append(t.Subtasks, sub)
	t.Done = false
	s.persist()
	return &t.Subtasks[len(t.Subtasks)-1]
}

func (s *Service) SetSubTitle(taskID, subID, title string) bool {
	t, _ := s.findTask(taskID)
	v := strings.TrimSpace(title)
	if t == nil || v == "" {
		return false
	}
	for i := range t.Subtasks {
		if t.Subtasks[i].ID == subID {
			t.Subtasks[i].Title = v
			s.persist()
			return true
		}
	}
	return false
}

func (s *Service) ToggleSubtask(taskID, subID string) (justCompleted bool, allDone bool, ok bool) {
	t, _ := s.findTask(taskID)
	if t == nil {
		return false, false, false
	}
	hit := -1
	for i := range t.Subtasks {
		if t.Subtasks[i].ID == subID {
			hit = i
			break
		}
	}
	if hit < 0 {
		return false, false, false
	}
	t.Subtasks[hit].Done = !t.Subtasks[hit].Done
	allDone = len(t.Subtasks) > 0
	for _, sub := range t.Subtasks {
		if !sub.Done {
			allDone = false
			break
		}
	}
	if allDone && !t.Done {
		t.Done = true
		justCompleted = true
	} else if !allDone {
		t.Done = false
	}
	s.persist()
	return justCompleted, allDone, true
}

func (s *Service) DeleteSubtask(taskID, subID string) bool {
	t, _ := s.findTask(taskID)
	if t == nil {
		return false
	}
	idx := -1
	for i := range t.Subtasks {
		if t.Subtasks[i].ID == subID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	t.Subtasks = append(t.Subtasks[:idx], t.Subtasks[idx+1:]...)
	if len(t.Subtasks) > 0 {
		all := true
		for _, sub := range t.Subtasks {
			if !sub.Done {
				all = false
				break
			}
		}
		t.Done = all
	}
	s.persist()
	return true
}

func (s *Service) AddDaily(title string) *store.DailyTemplate {
	v := strings.TrimSpace(title)
	if v == "" {
		return nil
	}
	cfg := s.DailyConfig()
	dt := store.DailyTemplate{ID: store.RandID("d"), Title: v}
	cfg.Tasks = append(cfg.Tasks, dt)
	if r := s.st.SetJSON(store.KeyDaily, cfg); !r.OK {
		return nil
	}
	s.SyncDaily()
	return &dt
}

func (s *Service) RenameDaily(id, title string) bool {
	v := strings.TrimSpace(title)
	if v == "" {
		return false
	}
	cfg := s.DailyConfig()
	old := ""
	found := false
	for i := range cfg.Tasks {
		if cfg.Tasks[i].ID == id {
			old = cfg.Tasks[i].Title
			cfg.Tasks[i].Title = v
			found = true
			break
		}
	}
	if !found {
		return false
	}
	s.st.SetJSON(store.KeyDaily, cfg)
	dd := s.day(s.TodayKey())
	touched := false
	for i := range dd.Tasks {
		if dd.Tasks[i].DailyID == id && dd.Tasks[i].Title == old {
			dd.Tasks[i].Title = v
			touched = true
		}
	}
	if touched {
		s.persist()
	}
	return true
}

func (s *Service) RemoveDaily(id string) bool {
	cfg := s.DailyConfig()
	out := make([]store.DailyTemplate, 0, len(cfg.Tasks))
	removed := false
	for _, t := range cfg.Tasks {
		if t.ID == id {
			removed = true
			continue
		}
		out = append(out, t)
	}
	if !removed {
		return false
	}
	s.st.SetJSON(store.KeyDaily, store.DailyConfig{Tasks: out})
	return true
}

func (s *Service) ViewPrefs() store.TodoViewPrefs {
	var vp store.TodoViewPrefs
	if s.st.GetJSON(store.KeyTodoView, &vp) {
		if vp.Collapsed == nil {
			vp.Collapsed = []string{}
		}
		return vp
	}
	return store.TodoViewPrefs{Collapsed: []string{}}
}

func (s *Service) SaveViewPrefs(vp store.TodoViewPrefs) {
	if vp.Collapsed == nil {
		vp.Collapsed = []string{}
	}
	s.st.SetJSON(store.KeyTodoView, vp)
}
