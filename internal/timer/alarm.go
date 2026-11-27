package timer

import (
	"fmt"
	"sync"
	"time"

	"github.com/Stars76/Xi/internal/dateutil"
	"github.com/Stars76/Xi/internal/store"
)

const AlarmScanInterval = 15 * time.Second

// AlarmScheduler scans today's unfinished tasks for matching HH:MM alarms.
type AlarmScheduler struct {
	mu      sync.Mutex
	store   *store.Store
	log     *store.Logger
	now     func() time.Time
	notify  func(title, body string)
	fired   map[string]bool
	lastDay string
	ticker  *time.Ticker
	stopCh  chan struct{}
}

func NewAlarmScheduler(st *store.Store, logger *store.Logger, notify func(title, body string), nowFn ...func() time.Time) *AlarmScheduler {
	nf := time.Now
	if len(nowFn) > 0 && nowFn[0] != nil {
		nf = nowFn[0]
	}
	return &AlarmScheduler{
		store:  st,
		log:    logger,
		now:    nf,
		notify: notify,
		fired:  map[string]bool{},
	}
}

func (a *AlarmScheduler) Start() {
	a.mu.Lock()
	if a.ticker != nil {
		a.mu.Unlock()
		return
	}
	a.lastDay = dateutil.TodayKey(a.now())
	a.fired = map[string]bool{}
	a.ticker = time.NewTicker(AlarmScanInterval)
	a.stopCh = make(chan struct{})
	t := a.ticker
	stop := a.stopCh
	a.mu.Unlock()

	a.Tick()
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				a.Tick()
			}
		}
	}()
}

func (a *AlarmScheduler) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopCh != nil {
		close(a.stopCh)
		a.stopCh = nil
	}
	if a.ticker != nil {
		a.ticker.Stop()
		a.ticker = nil
	}
}

func (a *AlarmScheduler) Tick() {
	var state store.FocusState
	if !a.store.GetJSON(store.KeyStateV2, &state) || state.Days == nil {
		return
	}
	d := a.now().Local()
	day := dateutil.TodayKey(d)
	hm := fmt.Sprintf("%02d:%02d", d.Hour(), d.Minute())

	a.mu.Lock()
	if a.lastDay != day {
		a.lastDay = day
		a.fired = map[string]bool{}
	}
	bucket := state.Days[day]
	if bucket == nil {
		a.mu.Unlock()
		return
	}
	var due []store.Task
	for _, t := range bucket.Tasks {
		if t.ID == "" || t.Alarm == "" || t.Done || t.Alarm != hm {
			continue
		}
		key := day + "/" + t.Alarm + "/" + t.ID
		if a.fired[key] {
			continue
		}
		a.fired[key] = true
		due = append(due, t)
	}
	a.mu.Unlock()

	for _, t := range due {
		title := t.Title
		if title == "" {
			title = "这件事"
		}
		if a.notify != nil {
			a.notify("⏰ 闹钟："+title, "到点了，该做「"+title+"」了")
		}
		if a.log != nil {
			a.log.Info("闹钟触发", t.ID, t.Alarm)
		}
	}
}
