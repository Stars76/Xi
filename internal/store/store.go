package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MaxBackups    = 10
	FlushDebounce = 250 * time.Millisecond
)

var backupFileRe = regexp.MustCompile(`^sisy-store-.*\.json$`)

// FS abstracts filesystem operations for fault-injection testing.
type FS interface {
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm os.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(path string) error
	ReadDir(path string) ([]os.DirEntry, error)
	Stat(path string) (os.FileInfo, error)
	CopyFile(src, dst string) error
}

type osFS struct{}

func (osFS) MkdirAll(p string, perm os.FileMode) error { return os.MkdirAll(p, perm) }
func (osFS) ReadFile(p string) ([]byte, error)         { return os.ReadFile(p) }
func (osFS) WriteFile(p string, d []byte, perm os.FileMode) error {
	return os.WriteFile(p, d, perm)
}
func (osFS) Rename(a, b string) error                 { return os.Rename(a, b) }
func (osFS) Remove(p string) error                    { return os.Remove(p) }
func (osFS) ReadDir(p string) ([]os.DirEntry, error)  { return os.ReadDir(p) }
func (osFS) Stat(p string) (os.FileInfo, error)       { return os.Stat(p) }
func (osFS) CopyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

type LoadResult struct {
	Loaded    bool
	Recovered string
	Backup    string
	Corrupt   bool
	Schema    int
	Migrated  []string
}

type SetResult struct {
	OK    bool
	Error string
}

type ImportResult struct {
	OK       bool
	Applied  int
	Mode     string
	Backup   string
	Summary  *ImportSummary
	Repaired []string
	Verified bool
	Error    string
	Errors   []string
}

type ClearResult struct {
	OK        bool
	Removed   int
	PreBackup string
	Error     string
}

type BackupResult struct {
	OK    bool
	Path  string
	Error string
}

type RecoveryInfo struct {
	Corrupt []string `json:"corrupt"`
	Temp    []string `json:"temp"`
	Backups []string `json:"backups"`
}

// Store is the single-writer authoritative data store backed by <dir>/sisy-store.json.
type Store struct {
	mu                sync.Mutex
	dir               string
	file              string
	backupDir         string
	fs                FS
	log               *Logger
	data              map[string]json.RawMessage
	dirty             bool
	flushTimer        *time.Timer
	lastError         string
	appliedMigrations []string
}

func NewStore(dir string, logger *Logger, customFS ...FS) *Store {
	var fsys FS = osFS{}
	if len(customFS) > 0 && customFS[0] != nil {
		fsys = customFS[0]
	}
	return &Store{
		dir:       dir,
		file:      filepath.Join(dir, "sisy-store.json"),
		backupDir: filepath.Join(dir, "backups"),
		fs:        fsys,
		log:       logger,
		data:      map[string]json.RawMessage{},
	}
}

func (s *Store) Dir() string       { return s.dir }
func (s *Store) File() string      { return s.file }
func (s *Store) BackupDir() string { return s.backupDir }
func (s *Store) LastError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

func (s *Store) MigrationLog() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.appliedMigrations)
}

func storeShapeOk(raw []byte) bool {
	m, ok := decodeObject(json.RawMessage(raw))
	if !ok {
		return false
	}
	if dRaw, hasData := m["data"]; hasData {
		_, dok := decodeObject(dRaw)
		return dok
	}
	for k := range m {
		if k != "schema" && k != "updatedAt" && !strings.HasPrefix(k, "sisy-") {
			return false
		}
	}
	return true
}

func candidateSemanticallyValid(raw []byte) bool {
	if !storeShapeOk(raw) {
		return false
	}
	m, _ := decodeObject(json.RawMessage(raw))
	dataObj := m
	if dRaw, hasData := m["data"]; hasData {
		dataObj, _ = decodeObject(dRaw)
	}
	if dataObj == nil {
		return false
	}
	for k, v := range dataObj {
		if !IsValidStoreKey(k) {
			return false
		}
		if IsCorruptKey(k) {
			continue
		}
		vr := ValidateValue(k, v)
		if !vr.OK {
			return false
		}
	}
	return true
}

func (s *Store) sanitizeKnownKeysLocked() bool {
	changed := false
	for _, known := range KnownKeys {
		val, exists := s.data[known]
		if !exists {
			continue
		}
		vr := ValidateValue(known, val)
		if !vr.OK {
			s.quarantineKeyLocked(known, val, strings.Join(vr.Errors, "; "))
			changed = true
			continue
		}
		if len(vr.Repaired) > 0 {
			if s.log != nil {
				s.log.Info("加载修复：", strings.Join(vr.Repaired, " | "))
			}
			s.data[known] = vr.Raw
			changed = true
		}
	}
	return changed
}

func (s *Store) quarantineKeyLocked(key string, val json.RawMessage, why string) {
	if len(why) > 200 {
		why = why[:200]
	}
	stamp := strings.NewReplacer(":", "-", ".", "-").Replace(time.Now().UTC().Format(time.RFC3339Nano))
	side := key + "__corrupt_" + stamp
	s.data[side] = mustMarshal(map[string]any{
		"raw":         string(val),
		"recoveredAt": time.Now().UTC().Format(time.RFC3339),
		"reason":      why,
	})
	delete(s.data, key)
	if s.log != nil {
		s.log.Warn("键", key, "数据非法（"+why+"），已另存为", side)
	}
}

func (s *Store) adoptParsedLocked(raw []byte) {
	env := MigrateEnvelope(json.RawMessage(raw))
	for _, n := range env.Notes {
		if s.log != nil {
			s.log.Info("迁移：", n)
		}
	}
	s.appliedMigrations = append(s.appliedMigrations, env.Notes...)
	s.data = env.Data
	sanitized := s.sanitizeKnownKeysLocked()
	if len(env.Notes) > 0 || sanitized {
		s.dirty = true
	}
}

func (s *Store) findRestorableBackupLocked() (string, []byte) {
	entries, err := s.fs.ReadDir(s.backupDir)
	if err != nil {
		return "", nil
	}
	type item struct {
		name  string
		mtime time.Time
	}
	var list []item
	for _, ent := range entries {
		if ent.IsDir() || !backupFileRe.MatchString(ent.Name()) {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		list = append(list, item{name: ent.Name(), mtime: info.ModTime()})
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].mtime.Equal(list[j].mtime) {
			return list[i].mtime.After(list[j].mtime)
		}
		return list[i].name > list[j].name
	})
	for _, it := range list {
		p := filepath.Join(s.backupDir, it.name)
		raw, err := s.fs.ReadFile(p)
		if err != nil {
			continue
		}
		if candidateSemanticallyValid(raw) {
			return p, raw
		}
	}
	return "", nil
}

func (s *Store) adoptFromCandidatesLocked(mainCorrupt bool) *LoadResult {
	tmpPath := s.file + ".tmp"
	if tmpRaw, err := s.fs.ReadFile(tmpPath); err == nil {
		if candidateSemanticallyValid(tmpRaw) {
			s.adoptParsedLocked(tmpRaw)
			s.dirty = true
			s.scheduleFlushLocked(0)
			if s.log != nil {
				s.log.Info("已从 .tmp 恢复（主文件缺失/损坏）")
			}
			return &LoadResult{Loaded: true, Recovered: "tmp", Corrupt: mainCorrupt}
		}
		side := strings.TrimSuffix(s.file, ".json") + fmt.Sprintf(".tmp-invalid-%d.json", time.Now().UnixMilli())
		_ = s.fs.WriteFile(side, tmpRaw, 0o644)
		if s.log != nil {
			s.log.Warn("tmp 文件无效，已另存", filepath.Base(side), "，尝试备份")
		}
	}
	if bakPath, bakRaw := s.findRestorableBackupLocked(); bakPath != "" {
		s.adoptParsedLocked(bakRaw)
		s.dirty = true
		s.scheduleFlushLocked(0)
		if s.log != nil {
			s.log.Info("已从备份恢复：", filepath.Base(bakPath))
		}
		return &LoadResult{Loaded: true, Recovered: "backup", Backup: bakPath, Corrupt: mainCorrupt}
	}
	return nil
}

// Load reads sisy-store.json or recovers from .tmp / rolling backups.
func (s *Store) Load() LoadResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.fs.MkdirAll(s.dir, 0o755)
	raw, err := s.fs.ReadFile(s.file)
	if err != nil {
		if rec := s.adoptFromCandidatesLocked(false); rec != nil {
			return *rec
		}
		s.data = map[string]json.RawMessage{}
		s.dirty = true
		s.scheduleFlushLocked(0)
		return LoadResult{Loaded: false}
	}
	if !storeShapeOk(raw) {
		side := strings.TrimSuffix(s.file, ".json") + fmt.Sprintf(".corrupt-%d.json", time.Now().UnixMilli())
		if werr := s.fs.WriteFile(side, raw, 0o644); werr != nil && s.log != nil {
			s.log.Warn("损坏现场另存失败", werr)
		}
		if s.log != nil {
			s.log.Warn("存储文件损坏，已另存为", filepath.Base(side), "，尝试恢复")
		}
		if rec := s.adoptFromCandidatesLocked(true); rec != nil {
			return *rec
		}
		if s.log != nil {
			s.log.Warn("没有可用备份，按空数据启动（损坏现场已保留）")
		}
		s.data = map[string]json.RawMessage{}
		s.dirty = true
		s.scheduleFlushLocked(0)
		return LoadResult{Loaded: false, Corrupt: true}
	}
	s.adoptParsedLocked(raw)
	if len(s.appliedMigrations) > 0 {
		s.dirty = true
	}
	s.scheduleFlushLocked(0)
	return LoadResult{Loaded: true, Schema: SchemaVersion, Migrated: slices.Clone(s.appliedMigrations)}
}

// RollingBackup creates a daily startup backup (up to MaxBackups files kept).
func (s *Store) RollingBackup() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.fs.Stat(s.file); err != nil {
		return ""
	}
	if err := s.fs.MkdirAll(s.backupDir, 0o755); err != nil {
		return ""
	}
	name := "sisy-store-" + time.Now().Format("20060102") + ".json"
	dest := filepath.Join(s.backupDir, name)
	if _, err := s.fs.Stat(dest); err == nil {
		return dest
	}
	if err := s.fs.CopyFile(s.file, dest); err != nil {
		if s.log != nil {
			s.log.Warn("启动备份失败", err)
		}
		return ""
	}
	s.pruneBackupsLocked()
	if s.log != nil {
		s.log.Info("已生成启动备份", name)
	}
	return dest
}

func (s *Store) pruneBackupsLocked() {
	entries, err := s.fs.ReadDir(s.backupDir)
	if err != nil {
		return
	}
	type item struct {
		name  string
		mtime time.Time
	}
	var list []item
	for _, ent := range entries {
		if ent.IsDir() || !backupFileRe.MatchString(ent.Name()) {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		list = append(list, item{name: ent.Name(), mtime: info.ModTime()})
	}
	sort.Slice(list, func(i, j int) bool {
		if !list[i].mtime.Equal(list[j].mtime) {
			return list[i].mtime.After(list[j].mtime)
		}
		return list[i].name > list[j].name
	})
	if len(list) > MaxBackups {
		for _, it := range list[MaxBackups:] {
			_ = s.fs.Remove(filepath.Join(s.backupDir, it.name))
		}
	}
}

func (s *Store) serializeLocked() ([]byte, error) {
	env := Envelope{
		Schema:    SchemaVersion,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Data:      s.data,
	}
	return json.MarshalIndent(env, "", "  ")
}

func (s *Store) writeNowLocked() error {
	if s.flushTimer != nil {
		s.flushTimer.Stop()
		s.flushTimer = nil
	}
	if !s.dirty {
		return nil
	}
	if err := s.fs.MkdirAll(s.dir, 0o755); err != nil {
		s.lastError = "目录不可写"
		return err
	}
	payload, err := s.serializeLocked()
	if err != nil {
		s.lastError = err.Error()
		return err
	}
	tmp := s.file + ".tmp"
	if err := s.fs.WriteFile(tmp, payload, 0o644); err != nil {
		s.lastError = err.Error()
		if s.log != nil {
			s.log.Warn("写入存储失败", err)
		}
		_ = s.fs.Remove(tmp)
		return err
	}
	if err := s.fs.Rename(tmp, s.file); err != nil {
		s.lastError = err.Error()
		if s.log != nil {
			s.log.Warn("原子替换存储失败", err)
		}
		_ = s.fs.Remove(tmp)
		return err
	}
	s.dirty = false
	s.lastError = ""
	return nil
}

func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeNowLocked()
}

func (s *Store) scheduleFlushLocked(delay time.Duration) {
	if s.flushTimer != nil {
		return
	}
	s.flushTimer = time.AfterFunc(delay, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.flushTimer = nil
		_ = s.writeNowLocked()
	})
}

func (s *Store) verifyOnDiskLocked() bool {
	raw, err := s.fs.ReadFile(s.file)
	if err != nil {
		return false
	}
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil || env.Data == nil {
		return false
	}
	a, err1 := canonicalJSON(env.Data)
	b, err2 := canonicalJSON(s.data)
	return err1 == nil && err2 == nil && bytes.Equal(a, b)
}

func canonicalJSON(data map[string]json.RawMessage) ([]byte, error) {
	norm := map[string]any{}
	for k, v := range data {
		var obj any
		if err := json.Unmarshal(v, &obj); err != nil {
			return nil, err
		}
		norm[k] = obj
	}
	return json.Marshal(norm)
}

func (s *Store) GetRaw(key string) (json.RawMessage, bool) {
	if !IsValidStoreKey(key) {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[key]
	if !ok {
		return nil, false
	}
	return bytes.Clone(v), true
}

func (s *Store) GetJSON(key string, dst any) bool {
	raw, ok := s.GetRaw(key)
	if !ok {
		return false
	}
	return json.Unmarshal(raw, dst) == nil
}

func (s *Store) SetRaw(key string, val json.RawMessage) SetResult {
	if !IsValidStoreKey(key) {
		return SetResult{Error: "非法键名"}
	}
	if len(val) == 0 {
		return SetResult{Error: "值不能为空"}
	}
	if IsKnownKey(key) {
		vr := ValidateValue(key, val)
		if !vr.OK {
			msg := strings.Join(vr.Errors[:min(3, len(vr.Errors))], "；")
			return SetResult{Error: "值不符合结构要求：" + msg}
		}
		if len(vr.Repaired) > 0 {
			val = vr.Raw
		}
	}
	if len(val) > MaxValueBytes {
		return SetResult{Error: "值超过 5MB 上限，拒绝写入"}
	}
	var chk any
	if err := json.Unmarshal(val, &chk); err != nil {
		return SetResult{Error: "值无法序列化: " + err.Error()}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = bytes.Clone(val)
	s.dirty = true
	s.scheduleFlushLocked(FlushDebounce)
	return SetResult{OK: true}
}

func (s *Store) SetJSON(key string, v any) SetResult {
	b, err := json.Marshal(v)
	if err != nil {
		return SetResult{Error: "值无法序列化: " + err.Error()}
	}
	return s.SetRaw(key, b)
}

func (s *Store) Remove(key string) SetResult {
	if !IsValidStoreKey(key) {
		return SetResult{Error: "非法键名"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; ok {
		delete(s.data, key)
		s.dirty = true
		s.scheduleFlushLocked(FlushDebounce)
	}
	return SetResult{OK: true}
}

func (s *Store) Keys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for k := range s.data {
		if IsValidStoreKey(k) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

func (s *Store) ExportAll() ExportBundle {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := make(map[string]json.RawMessage, len(s.data))
	for k, v := range s.data {
		snap[k] = bytes.Clone(v)
	}
	return ExportBundle{
		App:        "sisyphus",
		Schema:     SchemaVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Backend:    "main",
		Data:       snap,
	}
}

var tagRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func (s *Store) Backup(tag string) BackupResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.backupLocked(tag)
}

func (s *Store) backupLocked(tag string) BackupResult {
	if tag != "" && !tagRe.MatchString(tag) {
		tag = "manual"
	}
	if err := s.fs.MkdirAll(s.backupDir, 0o755); err != nil {
		return BackupResult{Error: "备份目录不可写"}
	}
	_ = s.writeNowLocked()
	stamp := time.Now().Format("20060102-150405")
	suffix := ""
	if tag != "" {
		suffix = "-" + tag
	}
	dest := filepath.Join(s.backupDir, fmt.Sprintf("sisy-store-%s%s.json", stamp, suffix))
	payload, err := s.serializeLocked()
	if err != nil {
		return BackupResult{Error: err.Error()}
	}
	if err := s.fs.WriteFile(dest, payload, 0o644); err != nil {
		return BackupResult{Error: err.Error()}
	}
	s.pruneBackupsLocked()
	return BackupResult{OK: true, Path: dest}
}

func cloneDataMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	dst := make(map[string]json.RawMessage, len(src))
	for k, v := range src {
		dst[k] = bytes.Clone(v)
	}
	return dst
}

// ImportAll validates, backs up once, writes, and verifies on disk with rollback on error.
func (s *Store) ImportAll(raw []byte, mode string) ImportResult {
	if mode == "" {
		mode = "merge"
	}
	if mode != "merge" && mode != "replace" {
		return ImportResult{Error: "非法导入模式：" + mode}
	}
	if len(raw) > MaxBundleBytes {
		return ImportResult{Error: "文件过大"}
	}
	val := ValidateBundle(json.RawMessage(raw))
	if !val.OK {
		errMsg := strings.Join(val.Errors, "；")
		if len([]rune(errMsg)) > 500 {
			errMsg = string([]rune(errMsg)[:500])
		}
		if s.log != nil {
			s.log.Warn("导入校验失败（未做任何改动）：", errMsg)
		}
		return ImportResult{Error: errMsg, Errors: val.Errors}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pre := s.backupLocked("pre-import")
	if !pre.OK {
		if s.log != nil {
			s.log.Warn("导入前备份失败，中止导入（数据未动）", pre.Error)
		}
		return ImportResult{Error: "导入前备份失败，已中止：" + pre.Error}
	}

	prev := cloneDataMap(s.data)
	applied := 0
	for k, v := range val.PerKey {
		s.data[k] = bytes.Clone(v)
		applied++
	}
	for k, v := range val.Corrupt {
		if IsCorruptKey(k) && IsValidStoreKey(k) {
			s.data[k] = bytes.Clone(v)
			applied++
		}
	}
	if mode == "replace" {
		for k := range s.data {
			if IsValidStoreKey(k) && !IsCorruptKey(k) {
				if _, keep := val.PerKey[k]; !keep {
					delete(s.data, k)
				}
			}
		}
	}
	changes := MigrateData(s.data)
	s.appliedMigrations = append(s.appliedMigrations, changes...)
	s.dirty = true
	if err := s.writeNowLocked(); err != nil {
		s.data = prev
		s.dirty = true
		_ = s.writeNowLocked()
		if s.log != nil {
			s.log.Warn("导入落盘失败，已回滚：", err)
		}
		return ImportResult{Error: "写入失败已回滚：" + err.Error(), Backup: pre.Path}
	}
	if !s.verifyOnDiskLocked() {
		_ = s.fs.CopyFile(pre.Path, s.file)
		if backRaw, rerr := s.fs.ReadFile(s.file); rerr == nil && storeShapeOk(backRaw) {
			s.adoptParsedLocked(backRaw)
		} else {
			s.data = prev
		}
		if s.log != nil {
			s.log.Warn("导入后验证失败，已从导入前备份回滚")
		}
		return ImportResult{Error: "导入后验证失败，已从备份回滚", Backup: pre.Path}
	}
	if s.log != nil {
		s.log.Info(fmt.Sprintf("导入完成 applied=%d mode=%s", applied, mode))
	}
	return ImportResult{
		OK:       true,
		Applied:  applied,
		Mode:     mode,
		Backup:   pre.Path,
		Summary:  val.Summary,
		Repaired: val.Repaired,
		Verified: true,
	}
}

// Clear removes keys resolved from scopes (nil = all business keys) after a pre-clear backup.
func (s *Store) Clear(scopes []string) ClearResult {
	req := ResolveClearScopes(scopes)
	if len(req.Bad) > 0 {
		return ClearResult{Error: "包含非法分区或键名：" + strings.Join(req.Bad, ", ")}
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	pre := s.backupLocked("pre-clear")
	if !pre.OK {
		return ClearResult{Error: "清空前备份失败：" + pre.Error}
	}
	var target []string
	if req.All || len(req.Keys) == 0 {
		for k := range s.data {
			if IsValidStoreKey(k) {
				target = append(target, k)
			}
		}
	} else {
		target = req.Keys
	}
	prev := cloneDataMap(s.data)
	removed := 0
	for _, k := range target {
		if _, exists := s.data[k]; exists {
			delete(s.data, k)
			removed++
		}
	}
	s.dirty = true
	if err := s.writeNowLocked(); err != nil {
		s.data = prev
		s.dirty = true
		_ = s.writeNowLocked()
		return ClearResult{Error: "清空失败已回滚：" + err.Error()}
	}
	return ClearResult{OK: true, Removed: removed, PreBackup: pre.Path}
}

func (s *Store) ListRecovery() RecoveryInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := RecoveryInfo{Corrupt: []string{}, Temp: []string{}, Backups: []string{}}
	if entries, err := s.fs.ReadDir(s.dir); err == nil {
		for _, ent := range entries {
			name := ent.Name()
			if strings.Contains(name, ".corrupt-") {
				out.Corrupt = append(out.Corrupt, name)
			}
			if strings.Contains(name, ".tmp") {
				out.Temp = append(out.Temp, name)
			}
		}
		slices.Sort(out.Corrupt)
		slices.Sort(out.Temp)
	}
	if entries, err := s.fs.ReadDir(s.backupDir); err == nil {
		type item struct {
			name  string
			mtime time.Time
		}
		var list []item
		for _, ent := range entries {
			if !ent.IsDir() && backupFileRe.MatchString(ent.Name()) {
				if info, ierr := ent.Info(); ierr == nil {
					list = append(list, item{name: ent.Name(), mtime: info.ModTime()})
				}
			}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].mtime.After(list[j].mtime) })
		for i := 0; i < len(list) && i < MaxBackups; i++ {
			out.Backups = append(out.Backups, list[i].name)
		}
	}
	return out
}
