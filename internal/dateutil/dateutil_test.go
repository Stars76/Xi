package dateutil

import (
	"testing"
	"time"
)

func TestNormalizeAndParseKey(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"2026-9-8", "2026-09-08"},
		{"2026-09-08", "2026-09-08"},
		{" 2026-11-5 ", "2026-11-05"},
		{"2026-02-29", ""}, // 2026 non-leap year
		{"2024-02-29", "2024-02-29"},
		{"2026-02-30", ""},
		{"2026-13-01", ""},
		{"invalid", ""},
	}
	for _, tc := range cases {
		if got := NormalizeKey(tc.in); got != tc.want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if IsValidKey("2026-9-8") {
		t.Error("unpadded key must not satisfy IsValidKey")
	}
	if !IsValidKey("2026-09-08") {
		t.Error("padded key must satisfy IsValidKey")
	}
}

func TestShiftDayAndLabels(t *testing.T) {
	if got := ShiftDay("2026-02-28", 1); got != "2026-03-01" {
		t.Fatalf("ShiftDay = %s", got)
	}
	if got := ShiftDay("2026-03-01", -1); got != "2026-02-28" {
		t.Fatalf("ShiftDay back = %s", got)
	}
	if got := DayLabel("2026-09-08", 2026); got != "9月8日" {
		t.Fatalf("DayLabel same year = %s", got)
	}
	if got := DayLabel("2025-09-08", 2026); got != "2025年9月8日" {
		t.Fatalf("DayLabel diff year = %s", got)
	}
	// 2026-09-08 is Tuesday
	if got := WeekdayLabel("2026-09-08"); got != "星期二" {
		t.Fatalf("WeekdayLabel = %s", got)
	}
}

func TestNormalizeAlarm(t *testing.T) {
	if got, ok := NormalizeAlarm("8:05"); !ok || got != "08:05" {
		t.Fatalf("NormalizeAlarm(8:05) = %q, %v", got, ok)
	}
	if got, ok := NormalizeAlarm("23:59"); !ok || got != "23:59" {
		t.Fatalf("NormalizeAlarm(23:59) = %q, %v", got, ok)
	}
	for _, bad := range []string{"", "24:00", "12:60", "abc"} {
		if _, ok := NormalizeAlarm(bad); ok {
			t.Errorf("NormalizeAlarm(%q) should fail", bad)
		}
	}
	_ = TodayKey(time.Now())
}
