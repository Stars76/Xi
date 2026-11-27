package dateutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	alarmRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)
	looseRe = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	strictRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	looseAlarmRe = regexp.MustCompile(`^(\d{1,2}):(\d{2})$`)
)

// TodayKey returns local YYYY-MM-DD for t.
func TodayKey(t time.Time) string {
	return t.Local().Format("2006-01-02")
}

// ParseKey strictly validates a zero-padded YYYY-MM-DD key in local calendar.
func ParseKey(s string) (time.Time, bool) {
	if !strictRe.MatchString(s) {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil || t.Format("2006-01-02") != s {
		return time.Time{}, false
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.Local), true
}

// NormalizeKey converts loose YYYY-M-D or YYYY-MM-DD into zero-padded YYYY-MM-DD.
// Invalid calendar dates (such as 2026-02-30) return "".
func NormalizeKey(s string) string {
	s = strings.TrimSpace(s)
	m := looseRe.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	if y < 1 || mo < 1 || mo > 12 || d < 1 || d > 31 {
		return ""
	}
	cand := fmt.Sprintf("%04d-%02d-%02d", y, mo, d)
	if _, ok := ParseKey(cand); !ok {
		return ""
	}
	return cand
}

// IsValidKey returns true iff s is a valid zero-padded YYYY-MM-DD date key.
func IsValidKey(s string) bool {
	_, ok := ParseKey(s)
	return ok
}

// ShiftDay returns a date key shifted by delta days from key.
func ShiftDay(key string, delta int) string {
	t, ok := ParseKey(key)
	if !ok {
		if nk := NormalizeKey(key); nk != "" {
			t, _ = ParseKey(nk)
		} else {
			t = time.Now()
		}
	}
	return t.AddDate(0, 0, delta).Format("2006-01-02")
}

// DayLabel formats a Chinese display label for key (e.g. "9月22日" or "2025年9月22日").
func DayLabel(key string, nowYear int) string {
	t, ok := ParseKey(key)
	if !ok {
		return key
	}
	if nowYear <= 0 {
		nowYear = time.Now().Year()
	}
	if t.Year() != nowYear {
		return fmt.Sprintf("%d年%d月%d日", t.Year(), int(t.Month()), t.Day())
	}
	return fmt.Sprintf("%d月%d日", int(t.Month()), t.Day())
}

// WeekdayLabel returns Chinese weekday label (e.g. "星期二").
func WeekdayLabel(key string) string {
	t, ok := ParseKey(key)
	if !ok {
		return ""
	}
	names := [...]string{"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六"}
	return names[int(t.Weekday())]
}

// NormalizeAlarm validates and normalizes "8:05" / "08:05" -> "08:05".
func NormalizeAlarm(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if m := looseAlarmRe.FindStringSubmatch(s); m != nil && len(m[1]) == 1 {
		s = "0" + m[1] + ":" + m[2]
	}
	if !alarmRe.MatchString(s) {
		return "", false
	}
	return s, true
}

// IsValidAlarm returns true iff s strictly matches HH:MM (00:00..23:59).
func IsValidAlarm(s string) bool {
	return alarmRe.MatchString(s)
}

// FmtDuration formats seconds as MM:SS or MMm.
func FmtDuration(sec int, showSeconds bool) string {
	if sec < 0 {
		sec = 0
	}
	if !showSeconds {
		m := (sec + 59) / 60
		return fmt.Sprintf("%02d:00", m)
	}
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

// FmtClock formats seconds into M:SS (no leading zero on single-digit minute, matching tray).
func FmtClock(sec int) string {
	if sec < 0 {
		sec = 0
	}
	return fmt.Sprintf("%d:%02d", sec/60, sec%60)
}
