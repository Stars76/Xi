package uiapp

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func at(hour int) time.Time { return time.Date(2026, 9, 9, hour, 0, 0, 0, time.Local) }

// Encounters belong to conditions: each one shows up only where it makes sense.
func TestEncounterHabitats(t *testing.T) {
	cases := []struct {
		name string
		kind string
		v    SeaView
		hour int
		want bool
	}{
		{"angler on a clear afternoon", EncounterAngler, SeaView{Weather: "clear"}, 14, true},
		{"angler in morning fog", EncounterAngler, SeaView{Weather: "fog"}, 7, true},
		{"no angler at night", EncounterAngler, SeaView{Weather: "clear"}, 23, false},
		{"no angler in a storm", EncounterAngler, SeaView{Weather: "storm"}, 14, false},
		{"pirates ride out a storm", EncounterPirate, SeaView{Weather: "storm"}, 11, true},
		{"pirates sail after dark", EncounterPirate, SeaView{Weather: "clear"}, 22, true},
		{"no pirates on a fair noon", EncounterPirate, SeaView{Weather: "clear"}, 12, false},
		{"lighthouse lit at night", EncounterLighthouse, SeaView{Weather: "clear"}, 23, true},
		{"lighthouse lit in daytime fog", EncounterLighthouse, SeaView{Weather: "fog"}, 13, true},
		{"lighthouse dark on a fair noon", EncounterLighthouse, SeaView{Weather: "clear"}, 12, false},
		{"aurora on a clear night after focus", EncounterAurora, SeaView{Weather: "clear", TodayMinutes: 60}, 23, true},
		{"no aurora without the day's focus", EncounterAurora, SeaView{Weather: "clear", TodayMinutes: 20}, 23, false},
		{"no aurora under cloud", EncounterAurora, SeaView{Weather: "rain", TodayMinutes: 120}, 23, false},
		{"no aurora by day", EncounterAurora, SeaView{Weather: "clear", TodayMinutes: 120}, 13, false},
		{"dolphins while working in fair weather", EncounterDolphin, SeaView{Weather: "clear", Running: true}, 15, true},
		{"no dolphins when idle with nothing done", EncounterDolphin, SeaView{Weather: "clear"}, 15, false},
	}
	for _, c := range cases {
		if got := encounterHabitat(c.kind, c.v, at(c.hour)) > 0; got != c.want {
			t.Errorf("%s: belongs=%v, want %v", c.name, got, c.want)
		}
	}

	// A snowy afternoon suits none of them: the sea is left to itself.
	var sr sceneRuntime
	if kind := pickWeightedEncounter(&sr, SeaView{Weather: "snow"}, at(14)); kind != "" {
		t.Errorf("expected no encounter on a snowy afternoon, got %q", kind)
	}
	// On a stormy night only storm- and night-dwellers can be drawn.
	for i := 0; i < 200; i++ {
		kind := pickWeightedEncounter(&sr, SeaView{Weather: "storm"}, at(23))
		if kind != EncounterPirate && kind != EncounterLighthouse {
			t.Fatalf("stormy night drew %q", kind)
		}
	}
}

// Weather, lighting and encounters all read the waterline from the frame the sea drew, and that
// line has to be where the sea's own height field crosses its surface threshold.
func TestSeaFrameSurfaceMatchesHeightField(t *testing.T) {
	for _, mood := range []seaMood{moodFamiliar, moodCalm, moodStorm, moodFine} {
		f := seaFrame{wl: 0.4, tt: 3.7, mood: mood, swellU: 0.5, swellAmt: 1, bumpU: 0.3, bumpAmt: 1}
		for _, u := range []float64{0, 0.13, 0.3, 0.5, 0.77, 1} {
			vv := f.surfaceVV(u)
			if h := f.height(u, vv); math.Abs(h-seaSurfaceH) > 0.004 {
				t.Errorf("mood %+v u=%.2f: height at solved surface = %.4f, want %.2f", mood, u, h, seaSurfaceH)
			}
		}
	}
	// A swell or something standing in the water raises the surface (smaller y) where it is.
	flat := seaFrame{wl: 0.4, tt: 1, mood: moodCalm}
	lifted := flat
	lifted.bumpU, lifted.bumpAmt = 0.5, 1
	r := ui.Rect{W: 800, H: 600}
	if !(lifted.surfaceY(r, 0.5) < flat.surfaceY(r, 0.5)) {
		t.Error("expected the water to stand higher over a bump")
	}
	if d := lifted.surfaceY(r, 0.95) - flat.surfaceY(r, 0.95); math.Abs(float64(d)) > 0.5 {
		t.Errorf("expected the bump to be local, far-away surface moved by %.2fpx", d)
	}
}

// Each weather has its own sea: storms move more water, faster, than fog.
func TestWeatherSetsTheSeaMood(t *testing.T) {
	storm, fog := seaMoodFor("storm", 10), seaMoodFor("fog", 10)
	if !(storm.Amp > fog.Amp && storm.Speed > fog.Speed) {
		t.Errorf("storm %+v should be rougher and faster than fog %+v", storm, fog)
	}
	if got, want := seaMoodFor("", 10), seaMoodFor("clear", 10); got != want {
		t.Errorf("unknown weather should fall back to clear: %+v vs %+v", got, want)
	}
	if _, amt := seaSwellAt("fog", 200); amt != 0 {
		t.Error("fog should never bring a swell")
	}
}
