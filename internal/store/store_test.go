package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type faultyFS struct {
	FS
	failWrite bool
	corruptVerify bool
	fileTarget string
}

func (f *faultyFS) WriteFile(p string, d []byte, perm os.FileMode) error {
	if f.failWrite && strings.HasSuffix(p, ".tmp") {
		return errors.New("injected disk write failure")
	}
	return f.FS.WriteFile(p, d, perm)
}

func (f *faultyFS) ReadFile(p string) ([]byte, error) {
	b, err := f.FS.ReadFile(p)
	if err == nil && f.corruptVerify && p == f.fileTarget {
		// Return stale/different stats in verification read
		return []byte(`{"schema":2,"data":{"sisy-timer-stats":{"day":"2026-09-09","rounds":999,"minutes":999}}}`), nil
	}
	return b, err
}

func TestImportValidationAndRollback(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(filepath.Join(dir, "logs"))
	s := NewStore(dir, logger)
	s.Load()
	if r := s.SetJSON(KeyTimerStats, TimerStats{Day: "2026-09-09", Rounds: 2, Minutes: 50}); !r.OK {
		t.Fatalf("seed stats failed: %+v", r)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}

	// Reject malformed / invalid bundles without creating backup
	bads := []string{
		`not json`,
		`{"foo":"bar"}`,
		`{"data":{"../evil":1}}`,
		`{"data":{"sisy-focus-state-v2":{"viewDay":"2026-09-09"}}}`,
		`{"data":{"sisy-timer-stats":{"day":"2026-09-09"}}}`,
		`{"data":{"sisy-timer-state":{"phase":"invalid","totalSec":60}}}`,
	}
	for _, raw := range bads {
		res := s.ImportAll([]byte(raw), "merge")
		if res.OK {
			t.Fatalf("expected failure for %s", raw)
		}
	}
	if rec := s.ListRecovery(); len(rec.Backups) != 0 {
		t.Fatalf("failed imports must not create backups, got %v", rec.Backups)
	}

	// Valid import with unpadded date key + unknown forward-compatible key
	good := `{
		"app": "sisyphus",
		"schema": 1,
		"data": {
			"sisy-focus-state-v2": {
				"viewDay": "2026-9-8",
				"days": {
					"2026-9-8": {
						"tasks": [{"id":"t1","title":"写代码","done":false,"subtasks":[],"alarm":"09:30","rolledFrom":"2026-9-7"}],
						"dismissedDaily": {}
					}
				}
			},
			"sisy-timer-prefs": {"theme": "dark"},
			"sisy-future-key": {"hello": 1}
		}
	}`
	okRes := s.ImportAll([]byte(good), "merge")
	if !okRes.OK || !okRes.Verified {
		t.Fatalf("valid import failed: %+v", okRes)
	}
	var fs FocusState
	if !s.GetJSON(KeyStateV2, &fs) || fs.ViewDay != "2026-09-08" || fs.Days["2026-09-08"].Tasks[0].RolledFrom != "2026-09-07" {
		t.Fatalf("expected normalized viewDay & rolledFrom, got %+v", fs)
	}
	if _, ok := s.GetRaw("sisy-future-key"); !ok {
		t.Fatal("unknown sisy-* key must be preserved")
	}

	// Rollback on write failure
	ffs := &faultyFS{FS: osFS{}, failWrite: true, fileTarget: filepath.Join(dir, "sisy-store.json")}
	sFault := NewStore(dir, logger, ffs)
	sFault.Load()
	ffs.failWrite = true
	failRes := sFault.ImportAll([]byte(`{"data":{"sisy-timer-stats":{"day":"2026-09-09","rounds":8,"minutes":200}}}`), "merge")
	if failRes.OK {
		t.Fatal("expected write failure rollback")
	}
	var st TimerStats
	if !sFault.GetJSON(KeyTimerStats, &st) || st.Rounds != 2 {
		t.Fatalf("expected stats rolled back to 2 rounds, got %+v", st)
	}
}

func TestCorruptRecoveryAndClearScopes(t *testing.T) {
	dir := t.TempDir()
	logger := NewLogger(filepath.Join(dir, "logs"))
	s := NewStore(dir, logger)
	s.Load()
	s.SetJSON(KeyTimerStats, TimerStats{Day: "2026-09-09", Rounds: 3, Minutes: 75})
	s.SetJSON(KeyDaily, DailyConfig{Tasks: []DailyTemplate{{ID: "d1", Title: "喝水"}}})
	_ = s.Flush()
	s.RollingBackup()

	// Corrupt main file
	_ = os.WriteFile(s.File(), []byte("{broken json"), 0o644)
	s2 := NewStore(dir, logger)
	rec := s2.Load()
	if !rec.Loaded || !rec.Corrupt || rec.Recovered != "backup" {
		t.Fatalf("expected backup recovery, got %+v", rec)
	}
	if list := s2.ListRecovery(); len(list.Corrupt) == 0 {
		t.Fatal("corrupt file snapshot must be preserved")
	}

	// Clear only stats scope; daily must survive
	clr := s2.Clear([]string{"stats"})
	if !clr.OK || clr.Removed != 1 {
		t.Fatalf("clear stats failed: %+v", clr)
	}
	if _, ok := s2.GetRaw(KeyTimerStats); ok {
		t.Fatal("stats should be cleared")
	}
	if _, ok := s2.GetRaw(KeyDaily); !ok {
		t.Fatal("daily config must remain untouched")
	}
	_ = s2.Flush()
}

func TestSchemaTimelineOverridesAndTemplates(t *testing.T) {
	ovRaw := json.RawMessage(`{
		"edits":[
			{"key":"ok","start":"08:30","hidden":true},
			{"key":"bad-empty","start":"99:99"}
		],
		"manual":[
			{"id":"m1","day":"2026-9-8","start":"09:00","end":"10:00","title":"手补"},
			{"id":"m2","day":"2026-09-08","start":"11:00","end":"10:00","title":"倒挂"}
		]
	}`)
	vr := ValidateValue(KeyTimelineOverrides, ovRaw)
	if !vr.OK {
		t.Fatalf("expected repaired overrides OK, errs=%v", vr.Errors)
	}
	var ov TimelineOverrides
	_ = json.Unmarshal(vr.Raw, &ov)
	if len(ov.Edits) != 1 || len(ov.Manual) != 1 || ov.Manual[0].Day != "2026-09-08" {
		t.Fatalf("unexpected repaired overrides: %+v", ov)
	}

	// Theme validation
	tv := ValidateValue(KeyTimerPrefs, json.RawMessage(`{"theme":"sepia","sound":true}`))
	if !tv.OK || len(tv.Repaired) == 0 {
		t.Fatalf("expected invalid theme repaired: %+v", tv)
	}
	var pr TimerPrefs
	_ = json.Unmarshal(tv.Raw, &pr)
	if pr.Theme != "" {
		t.Fatalf("invalid theme must be stripped, got %q", pr.Theme)
	}
}
