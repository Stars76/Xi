package uiapp

import (
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Encounter kind identifiers (all 6 are random ocean easter eggs).
const (
	EncounterAngler     = "angler"     // 垂钓小船
	EncounterPirate     = "pirate"     // 幽灵海盗船
	EncounterWhale      = "whale"      // 深海蓝鲸
	EncounterDolphin    = "dolphin"    // 跃浪海豚群
	EncounterLighthouse = "lighthouse" // 深夜灯塔与流星
	EncounterAurora     = "aurora"     // 海面字符极光
)

type EncounterMeta struct {
	Kind     string
	Name     string
	Subtitle string
	Preview  string
	Hint     string
}

// AllEncounters lists the six encounters. Hint states where each one really belongs (see
// encounterHabitat); the atlas shows it, so collecting them means knowing when to look.
var AllEncounters = []EncounterMeta{
	{Kind: EncounterAngler, Name: "垂钓小船", Preview: `  o  , /|\/ \___/`, Hint: "晴或雾的白天"},
	{Kind: EncounterPirate, Name: "海盗船", Preview: ` |~ /|\ \____/`, Hint: "雷雨中，或入夜后"},
	{Kind: EncounterWhale, Name: "蓝鲸", Preview: `.-""-. \|/ `+"`-.__.-'", Hint: "完成一轮专注"},
	{Kind: EncounterDolphin, Name: "海豚", Preview: ` _.-._ ,-' o `+"`>", Hint: "晴天专注时"},
	{Kind: EncounterLighthouse, Name: "灯塔", Preview: `/^\ [*]= = - -`, Hint: "夜里，或雾与雷雨中"},
	{Kind: EncounterAurora, Name: "极光", Preview: `' | | : ' | :`, Hint: "晴夜 · 当日专注满 50 分钟"},
}

// DayPeriod classifies local time into 4 restrained atmospheric periods.
func DayPeriod(now time.Time) (key string, label string) {
	h := now.Local().Hour()
	switch {
	case h >= 5 && h < 9:
		return "dawn", "晨光"
	case h >= 9 && h < 17:
		return "day", "晴昼"
	case h >= 17 && h < 20:
		return "dusk", "暮色"
	default:
		return "night", "星夜"
	}
}

type activeEncounter struct {
	kind       string
	startedAt  time.Time
	duration   time.Duration
	dir        float64 // +1 left->right, -1 right->left
	seed       uint32
	natural    bool
	reactAt    time.Time
	reactCount int
	bounds     ui.Rect
}

// sceneRuntime is owned by SeaState (guarded by s.mu) so Today & Focus Clock stay 100% synced.
type sceneRuntime struct {
	active       *activeEncounter
	nextSpawnAt  time.Time
	taskPulseAt  time.Time
	rng          uint32
	cachedPx     int
	glyphMap     map[rune][]ui.Glyph
	glyphAscent  float32
	glyphAdvance float32
	bigMap       map[bigKey]bigGlyph
	bumpU        float64 // where an encounter is lifting the water (0..1), and
	bumpAmt      float64 // how strongly (0..1); written while drawing encounters, read by the next frame's sea
}

type bigKey struct {
	r    rune
	size float32
}

type bigGlyph struct {
	gs  []ui.Glyph
	w   float32
	asc float32
}

func (sr *sceneRuntime) nextRand() float64 {
	if sr.rng == 0 {
		sr.rng = uint32(time.Now().UnixNano() ^ 0x9e3779b9)
		if sr.rng == 0 {
			sr.rng = 0x517cc1b7
		}
	}
	x := sr.rng
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	sr.rng = x
	return float64(x) / 4294967296.0
}

func (sr *sceneRuntime) ensureFont(px int) {
	if px < 8 {
		px = 8
	}
	if sr.cachedPx == px && sr.glyphMap != nil {
		return
	}
	sr.cachedPx = px
	sr.glyphMap = map[rune][]ui.Glyph{}
	f := ui.Font{Family: FontMono, Size: float32(px)}
	sr.glyphAscent = f.Metrics().Ascent
	ref := ui.Shape("M", f)
	var adv float32
	for _, g := range ref {
		adv += g.Advance
	}
	if adv <= 0 {
		adv = float32(px) * 0.6
	}
	sr.glyphAdvance = adv
}

// shapeBig shapes one rune at an arbitrary size (the sun and moon), cached per rune and size.
func (sr *sceneRuntime) shapeBig(r rune, size float32) (gs []ui.Glyph, width, ascent float32) {
	key := bigKey{r, size}
	if b, ok := sr.bigMap[key]; ok {
		return b.gs, b.w, b.asc
	}
	f := ui.Font{Family: FontMono, Size: size}
	b := bigGlyph{gs: ui.Shape(string(r), f), asc: f.Metrics().Ascent}
	for _, g := range b.gs {
		b.w += g.Advance
	}
	if sr.bigMap == nil {
		sr.bigMap = map[bigKey]bigGlyph{}
	}
	sr.bigMap[key] = b
	return b.gs, b.w, b.asc
}

func (sr *sceneRuntime) shapeRune(r rune) []ui.Glyph {
	if gs, ok := sr.glyphMap[r]; ok {
		return gs
	}
	f := ui.Font{Family: FontMono, Size: float32(sr.cachedPx)}
	gs := ui.Shape(string(r), f)
	sr.glyphMap[r] = gs
	return gs
}

// PulseTaskWave triggers an ascending luminous ocean ripple when a task is completed.
func (s *SeaState) PulseTaskWave(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	s.scene.taskPulseAt = now
	s.mu.Unlock()
}

func encounterDuration(kind string) time.Duration {
	switch kind {
	case EncounterAngler:
		return 26 * time.Second
	case EncounterWhale:
		return 16 * time.Second
	case EncounterDolphin:
		return 18 * time.Second
	case EncounterLighthouse:
		return 24 * time.Second
	case EncounterAurora:
		return 22 * time.Second
	default:
		return 20 * time.Second
	}
}

// encounterHabitat reports how likely kind is to turn up in the conditions of v at now; 0 means it
// does not belong here at all. This is what makes an encounter a consequence of the weather, the
// hour and the day's work rather than a roll of the dice.
func encounterHabitat(kind string, v SeaView, now time.Time) float64 {
	period, _ := DayPeriod(now)
	weather := seaWeatherKey(v.Weather)
	night := period == "night"
	clear := weather == "clear"
	switch kind {
	case EncounterAngler: // still water by day: clear or misty, never at night
		if !night && (clear || weather == "fog") {
			return 30
		}
	case EncounterPirate: // rides out storms, and sails after dark
		if weather == "storm" {
			return 34
		}
		if (period == "dusk" || night) && (clear || weather == "rain") {
			return 16
		}
	case EncounterLighthouse: // lit when it is needed: at night, in fog, in a storm
		if weather == "fog" || weather == "storm" {
			return 30
		}
		if night {
			return 24
		}
	case EncounterAurora: // a clear night, after a solid day's focus
		if night && clear && v.TodayMinutes >= auroraFocusMinutes {
			return 26
		}
	case EncounterDolphin: // fair weather, while you are at work
		if clear && !night && (v.Running || v.TodayRounds >= 2) {
			return 20
		}
	case EncounterWhale: // mostly comes when a round is finished (see onTimerEvent); rarely by chance
		if clear || weather == "rain" {
			return 5
		}
	}
	return 0
}

// auroraFocusMinutes is how much focus the day needs before the aurora will show at night.
const auroraFocusMinutes = 50

// pickWeightedEncounter chooses among the encounters that belong in the current conditions.
// It returns "" when none does, which simply leaves the sea to itself for a while.
func pickWeightedEncounter(sr *sceneRuntime, v SeaView, now time.Time) string {
	total := 0.0
	weights := make([]float64, len(AllEncounters))
	for k, e := range AllEncounters {
		weights[k] = encounterHabitat(e.Kind, v, now)
		total += weights[k]
	}
	if total <= 0 {
		return ""
	}
	roll := sr.nextRand() * total
	for k, e := range AllEncounters {
		if roll < weights[k] {
			return e.Kind
		}
		roll -= weights[k]
	}
	return AllEncounters[len(AllEncounters)-1].Kind
}

// TriggerEncounter immediately spawns a specific (or random if empty) ocean easter egg.
func (s *SeaState) TriggerEncounter(kind string, now time.Time, natural bool) string {
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if kind == "" {
		kinds := []string{EncounterAngler, EncounterPirate, EncounterWhale, EncounterDolphin, EncounterLighthouse, EncounterAurora}
		kind = kinds[int(s.scene.nextRand()*float64(len(kinds)))%len(kinds)]
	}
	dur := encounterDuration(kind)
	dir := 1.0
	if (kind == EncounterPirate || kind == EncounterWhale || kind == EncounterDolphin) && s.scene.nextRand() < 0.48 {
		dir = -1.0
	}
	seed := uint32(now.UnixNano() ^ 0x27d4eb2d)
	s.scene.active = &activeEncounter{
		kind:      kind,
		startedAt: now,
		duration:  dur,
		dir:       dir,
		seed:      seed,
		natural:   natural,
	}
	s.scene.nextSpawnAt = now.Add(dur + time.Duration(45+int(s.scene.nextRand()*55))*time.Second)
	return kind
}

// HandleOceanClick checks if (px, py) hit the active ocean encounter and triggers its audio-visual micro-reaction.
func (s *SeaState) HandleOceanClick(px, py float32, now time.Time) (kind string, soundKind string, caughtFish bool) {
	if now.IsZero() {
		now = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	act := s.scene.active
	if act == nil || now.Sub(act.startedAt) >= act.duration {
		return "", "", false
	}
	b := act.bounds
	if b.W <= 0 || b.H <= 0 {
		return "", "", false
	}
	pad := float32(28)
	if px < b.X-pad || px > b.X+b.W+pad || py < b.Y-pad || py > b.Y+b.H+pad {
		return "", "", false
	}

	act.reactAt = now
	act.reactCount++
	switch act.kind {
	case EncounterAngler:
		return act.kind, "droplet", true
	case EncounterWhale, EncounterDolphin:
		return act.kind, "droplet", false
	default:
		return act.kind, "chime", false
	}
}

// ActiveEncounterKind returns the currently active encounter kind (or "" if none).
func (s *SeaState) ActiveEncounterKind(now time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scene.active == nil {
		return ""
	}
	if now.Sub(s.scene.active.startedAt) >= s.scene.active.duration {
		return ""
	}
	return s.scene.active.kind
}

// ClearEncounter removes any active encounter immediately.
func (s *SeaState) ClearEncounter() {
	s.mu.Lock()
	s.scene.active = nil
	s.mu.Unlock()
}

// drawTaskPulse renders an ascending luminous wave ring across the sea when a user checks off a task.
func (s *SeaState) drawTaskPulse(p *ui.Painter, r ui.Rect, v SeaView, now time.Time) bool {
	if v.ReduceMotion || v.Motion == "off" {
		return false
	}
	s.mu.Lock()
	pulseAt := s.scene.taskPulseAt
	s.mu.Unlock()
	if pulseAt.IsZero() {
		return false
	}
	elapsed := now.Sub(pulseAt).Seconds()
	const pulseDur = 1.05
	if elapsed < 0 || elapsed >= pulseDur {
		return false
	}
	frac := elapsed / pulseDur
	ease := 1 - (1-frac)*(1-frac)
	bandY := r.Y + r.H*(1.0-float32(ease)*0.92)
	bandH := float32(math.Max(28, float64(r.H)*0.24))
	alpha := float32(math.Sin(frac*math.Pi)) * 0.24
	col := ui.Hex("#007CFF")
	if v.Dark {
		col = ui.Hex("#66B5FF")
	}
	p.FillGradient(ui.Rect{X: r.X, Y: bandY - bandH/2, W: r.W, H: bandH}, ui.LinearGradient{
		Angle: 180,
		Stops: []ui.GradientStop{
			{Color: ui.Transparent, At: 0.0},
			{Color: col.Alpha(alpha), At: 0.5},
			{Color: ui.Transparent, At: 1.0},
		},
	}, 0)
	return true
}

// seaWaveSurfaceY returns the live wave-surface Y coordinate at horizontal fraction xNorm,
// locked to flow.go's exact wave phase so raindrops, fog banks, moonlight reflections & encounters match the waves.
// stormFlash is the brightness (0..1) of the lightning flash at wave phase tt, or 0 between strikes.
// The sea and the bolt both read it, so the water flares exactly when the bolt shows.
func stormFlash(v SeaView, tt float64) float64 {
	if v.Weather != "storm" || v.Motion == "off" || v.ReduceMotion || v.HighContrast {
		return 0
	}
	cycle := math.Mod(tt, stormStrikeEvery)
	if cycle >= stormStrikeLen {
		return 0
	}
	t := cycle / stormStrikeLen
	// Double flicker, like a real strike
	return math.Sin(t*math.Pi) * (0.65 + 0.35*math.Abs(math.Sin(t*math.Pi*3.0)))
}

const (
	stormStrikeEvery = 8.8  // wave-phase units between strikes
	stormStrikeLen   = 0.34 // how long one strike lasts
)

// seaLight describes the sun or moon for the current hour: where it stands, what colour its light
// is, and how strongly it lies on the water. Strength is 0 when there is none to see.
type seaLight struct {
	period   string  // dawn | day | dusk | night
	u        float64 // horizontal position, 0..1
	low      bool    // near the horizon (dawn, dusk) rather than high in the sky
	col      ui.Color
	strength float64
}

// sceneLight works out the light for v at now. Weather owns the sky: under rain, storm, fog or snow
// there is no sun, moon or star to see, and nothing to reflect.
func sceneLight(v SeaView, now time.Time) seaLight {
	period, _ := DayPeriod(now)
	l := seaLight{period: period}
	if !v.TimeLighting || v.HighContrast || (v.Weather != "" && v.Weather != "clear") {
		return l
	}
	warm, moon, noon := ui.Hex("#E9A23B"), ui.Hex("#007CFF"), ui.Hex("#4DABFF")
	if v.Dark {
		warm, moon, noon = ui.Hex("#FFD79A"), ui.Hex("#EBF5FF"), ui.Hex("#BFE2FF")
	}
	switch period {
	case "dawn":
		l.u, l.low, l.col, l.strength = 0.70, true, warm, 0.85
	case "dusk":
		l.u, l.low, l.col, l.strength = 0.30, true, warm, 0.80
	case "night":
		l.u, l.col, l.strength = 0.36, moon, 0.90
	default:
		l.u, l.col, l.strength = 0.36, noon, 0.45
	}
	return l
}

// drawSceneBackdrop writes the sky in characters on the same flat paper as the sea: a sun or moon,
// stars at night, a few cloud outlines by day, birds at dusk. There is no painted sky; the hour is
// told by what is up there and by its light lying on the water (see the light path in SeaState.Draw).
func (s *SeaState) drawSceneBackdrop(p *ui.Painter, r ui.Rect, v SeaView, wl float64, tt float64, now time.Time) {
	light := sceneLight(v, now)
	if light.strength <= 0 {
		return
	}
	horizon := s.frame.surfaceY(r, 0.5)
	skyH := horizon - r.Y
	if skyH < 112 {
		return // the water stands too high to leave a sky
	}
	still := v.Motion == "off" || v.ReduceMotion

	px := 11
	if r.W >= 540 {
		px = 12
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scene.ensureFont(px)
	adv := s.scene.glyphAdvance
	asc := s.scene.glyphAscent
	glyph := func(ch rune, x, y float32, col ui.Color) {
		p.Glyphs(s.scene.shapeRune(ch), x-adv/2, y+asc*0.5, col)
	}
	big := func(ch rune, size, x, y float32, col ui.Color) {
		gs, w, a := s.scene.shapeBig(ch, size)
		p.Glyphs(gs, x-w/2, y+a*0.36, col)
	}

	ink := ui.Hex("#0B4F96")
	soft := ui.Hex("#4DABFF")
	if v.Dark {
		ink, soft = ui.Hex("#CBE6FF"), ui.Hex("#6CB5FF")
	}

	// Sun or moon: high in the sky by day and night, sitting just above the horizon at dawn and dusk.
	bx := r.X + float32(light.u)*r.W
	// Kept below the strip at the top where the page's own header sits.
	by := r.Y + float32(math.Max(84, float64(skyH)*0.42))
	if light.low {
		by = s.frame.surfaceY(r, light.u) - 30
		if by < r.Y+24 {
			by = r.Y + 24
		}
	}
	body := '○'
	if light.period == "night" {
		body = '☾'
	}
	big(body, 26, bx, by, light.col.Alpha(0.95))

	switch light.period {
	case "night":
		count := 14
		if r.W >= 540 {
			count = 24
		}
		stars := []rune{'✦', '+', '·', '·'}
		for i := 0; i < count; i++ {
			sx := r.X + float32(0.05+0.90*cellHash(i*13+5, 3))*r.W
			sy := r.Y + 14 + float32(0.02+0.78*cellHash(i*7+2, 11))*(skyH-40)
			if math.Abs(float64(sx-bx)) < 26 && math.Abs(float64(sy-by)) < 24 {
				continue // leave the moon its room
			}
			tw := 0.50
			if !still {
				tw = 0.25 + 0.55*(0.5+0.5*math.Sin(tt*1.8+float64(i)*1.7))
			}
			glyph(stars[i%len(stars)], sx, sy, ink.Alpha(float32(tw)))
		}
	case "day":
		// A few cloud outlines drifting: a row of arcs, nothing filled in.
		clouds := 2
		if r.W >= 540 {
			clouds = 3
		}
		for i := 0; i < clouds; i++ {
			spd := 0.010 + float64(i%2)*0.004
			drift := math.Mod(float64(i)*0.37+tt*spd+0.12, 1.30) - 0.15
			cx := r.X + float32(drift)*r.W
			cy := r.Y + 74 + float32(0.10+0.30*float64(i%3)/2)*(skyH-100)
			arcs := 3 + i%3
			for k := 0; k < arcs; k++ {
				x := cx + (float32(k)-float32(arcs)/2)*adv*1.9
				if math.Abs(float64(x-bx)) < 24 && math.Abs(float64(cy-by)) < 20 {
					continue
				}
				lift := float32(0)
				if k > 0 && k < arcs-1 {
					lift = -3 // the middle of a cloud stands a little taller
				}
				glyph('⌒', x, cy+lift, soft.Alpha(0.70))
			}
		}
	case "dusk":
		// Birds heading home: v, -, ^ in turn make the wingbeat.
		birds := 3
		if r.W >= 540 {
			birds = 5
		}
		wing := []rune{'v', '-', '^', '-'}
		for i := 0; i < birds; i++ {
			spd := 0.022 + float64(i%3)*0.006
			drift := math.Mod(float64(i)*0.21+tt*spd+0.12, 1.22) - 0.11
			x := r.X + float32(drift)*r.W
			y := r.Y + 72 + float32(0.08+0.20*float64(i%3))*(skyH-96) + float32(2.5*math.Sin(tt*1.6+float64(i)))
			beat := 0
			if !still {
				beat = int(math.Floor(tt*3.2+float64(i)*0.7)) % len(wing)
			}
			glyph(wing[beat], x, y, ink.Alpha(float32(0.62-0.08*float64(i%3))))
		}
	}
}

// drawSceneEncounters updates & renders active random ASCII easter eggs (all 6) + click reaction quotes!
func (s *SeaState) drawSceneEncounters(p *ui.Painter, r ui.Rect, v SeaView, wl float64, tt float64, now time.Time) bool {
	if v.HighContrast || v.Motion == "off" || r.W < 180 || r.H < 90 {
		return false
	}

	var spawnedNatural string
	var act *activeEncounter

	s.mu.Lock()
	if s.scene.active != nil && now.Sub(s.scene.active.startedAt) >= s.scene.active.duration {
		s.scene.active = nil
	}
	if v.EncountersOn && s.scene.active == nil {
		if s.scene.nextSpawnAt.IsZero() {
			s.scene.nextSpawnAt = now.Add(time.Duration(20+int(s.scene.nextRand()*18)) * time.Second)
		} else if !now.Before(s.scene.nextSpawnAt) {
			kind := pickWeightedEncounter(&s.scene, v, now)
			if kind == "" {
				// Nothing belongs in these conditions: look again in a while.
				s.scene.nextSpawnAt = now.Add(40 * time.Second)
			}
			dur := encounterDuration(kind)
			dir := 1.0
			if (kind == EncounterPirate || kind == EncounterWhale) && s.scene.nextRand() < 0.45 {
				dir = -1.0
			}
			if kind != "" {
				s.scene.active = &activeEncounter{
					kind:      kind,
					startedAt: now,
					duration:  dur,
					dir:       dir,
					seed:      uint32(now.UnixNano() ^ 0x5bf03635),
					natural:   true,
				}
				waitSec := 65 + int(s.scene.nextRand()*55)
				if v.Running {
					waitSec = 45 + int(s.scene.nextRand()*45)
				}
				s.scene.nextSpawnAt = now.Add(dur + time.Duration(waitSec)*time.Second)
				spawnedNatural = kind
			}
		}
	}
	if s.scene.active != nil {
		cp := *s.scene.active
		act = &cp
	}
	s.mu.Unlock()

	if spawnedNatural != "" && v.OnEncounter != nil {
		v.OnEncounter(spawnedNatural)
	}
	if act == nil {
		s.mu.Lock()
		s.scene.bumpAmt = 0 // nothing stands in the water any more
		s.mu.Unlock()
		return false
	}

	prog := now.Sub(act.startedAt).Seconds() / act.duration.Seconds()
	if prog < 0 {
		prog = 0
	}
	if prog > 1 {
		return false
	}

	edgeAlpha := 1.0
	if prog < 0.08 {
		edgeAlpha = prog / 0.08
	} else if prog > 0.90 {
		edgeAlpha = (1.0 - prog) / 0.10
	}

	reactAge := 99.0
	if !act.reactAt.IsZero() {
		reactAge = now.Sub(act.reactAt).Seconds()
	}
	reacting := reactAge >= 0 && reactAge < 2.2

	compact := r.H < 210 || r.W < 420
	// The resting waterline, for the few effects that want the sea's level rather than a wave on it.
	surfFrac := math.Max(0.05, math.Min(0.92, s.frame.surfaceVV(0.5)))

	px := 12
	if compact {
		px = 10
	} else if r.W >= 680 {
		px = 13
	}

	// Curated multi-tone palette strictly aligned with the Paper White x Ocean Blue / Deep Sea Night x Ice Blue
	hullCol := ui.Hex("#08335E")
	hullMid := ui.Hex("#165B9E")
	sailCol := ui.Hex("#2F8DF2")
	accentCol := ui.Hex("#007CFF")
	foamCol := ui.Hex("#58B2FF")
	lanternCol := ui.Hex("#F5A623")
	whaleBack := ui.Hex("#094B8E")
	whaleMid := ui.Hex("#1C7EE6")
	whaleHigh := ui.Hex("#5FB4FF")
	auroraA := ui.Hex("#2F94FF")
	auroraB := ui.Hex("#00B4CC")
	auroraC := ui.Hex("#38D9C5")
	if v.Dark {
		hullCol = ui.Hex("#D8ECFF")
		hullMid = ui.Hex("#8CC4FA")
		sailCol = ui.Hex("#66B5FF")
		accentCol = ui.Hex("#56A9FF")
		foamCol = ui.Hex("#93D1FF")
		lanternCol = ui.Hex("#FFD166")
		whaleBack = ui.Hex("#3B9BFF")
		whaleMid = ui.Hex("#78BFFF")
		whaleHigh = ui.Hex("#CBE7FF")
		auroraA = ui.Hex("#56AEFF")
		auroraB = ui.Hex("#2DD4BF")
		auroraC = ui.Hex("#7CF5E8")
	}

	s.mu.Lock()
	s.scene.ensureFont(px)
	adv := s.scene.glyphAdvance
	asc := s.scene.glyphAscent
	lineH := float32(px) * 1.05

	// Everything that floats reads the waterline the sea actually drew this frame, so it rises on
	// crests, dips into troughs and tilts with the slope under it — whatever the mood or swell.
	frame := s.frame
	waveSample := func(xNorm float64) (surfY float32, surgeX float32, slope float64) {
		surfY = frame.surfaceY(r, xNorm)
		slope = frame.surfaceSlope(r, xNorm)
		// Things rock with the water, but only gently: a character grid shears badly if tilted far.
		slope *= 0.45
		if slope > 0.09 {
			slope = 0.09
		} else if slope < -0.09 {
			slope = -0.09
		}
		surgeX = float32(-slope * 22) // a little horizontal surge down the face of the wave
		return surfY, surgeX, slope
	}

	// Every encounter is written in characters only, in full-strength ink, and stands in the real
	// water: it rides the waterline the sea drew, the sea is knocked out behind its body so it reads
	// as solid, and whatever is below the surface is simply not drawn.
	_, _, _, _, _ = whaleMid, whaleHigh, hullMid, compact, surfFrac
	paper := LightPalette.Paper
	if v.Dark {
		paper = DarkPalette.Paper
	}
	noClip := r.Y + r.H + 1000

	type sline struct {
		text string
		col  ui.Color
	}
	lines := func(col ui.Color, rows ...string) []sline {
		out := make([]sline, len(rows))
		for k, t := range rows {
			out[k] = sline{t, col}
		}
		return out
	}
	// mirror flips a sprite left-to-right, swapping the characters that have a handedness.
	mirror := func(in []sline) []sline {
		swap := map[rune]rune{'/': '\\', '\\': '/', '(': ')', ')': '(', '<': '>', '>': '<', '`': '\'', '\'': '`', '[': ']', ']': '['}
		w := 0
		for _, l := range in {
			if n := len([]rune(l.text)); n > w {
				w = n
			}
		}
		out := make([]sline, len(in))
		for k, l := range in {
			rs := []rune(l.text)
			for len(rs) < w {
				rs = append(rs, ' ')
			}
			for a, b := 0, len(rs)-1; a < b; a, b = a+1, b-1 {
				rs[a], rs[b] = rs[b], rs[a]
			}
			for a, ch := range rs {
				if m, ok := swap[ch]; ok {
					rs[a] = m
				}
			}
			out[k] = sline{string(rs), l.col}
		}
		return out
	}
	// sprite draws rows so that the bottom of the last row sits at baseY, tilted by slope around cx.
	// Cells that fall below clipY are under water and skipped.
	sprite := func(rows []sline, cx, baseY float32, slope float64, alpha float64, clipY float32) ui.Rect {
		w := 0
		for _, l := range rows {
			if n := len([]rune(l.text)); n > w {
				w = n
			}
		}
		bw := float32(w) * adv
		startX := cx - bw/2
		topY := baseY - float32(len(rows))*lineH
		a := float32(math.Min(1, alpha))
		if a <= 0.01 {
			return ui.Rect{}
		}
		// Two passes: first clear the sea behind every cell of the body, then write the characters,
		// so a row's knock-out can never wipe the underscores of the row above it.
		for pass := 0; pass < 2; pass++ {
			for ri, l := range rows {
				rs := []rune(l.text)
				first, last := -1, -1
				for k, ch := range rs {
					if ch != ' ' {
						if first < 0 {
							first = k
						}
						last = k
					}
				}
				for k := first; first >= 0 && k <= last; k++ {
					x := startX + float32(k)*adv
					y := topY + float32(ri)*lineH + float32(float64(x+adv/2-cx)*slope)
					if y+lineH*0.5 > clipY {
						continue
					}
					if pass == 0 {
						p.Fill(ui.Rect{X: x - 0.5, Y: y, W: adv + 1, H: lineH + 0.5}, paper.Alpha(a), 0)
					} else if rs[k] != ' ' {
						p.Glyphs(s.scene.shapeRune(rs[k]), x, y+asc, l.col.Alpha(a))
					}
				}
			}
		}
		return ui.Rect{X: startX, Y: topY, W: bw, H: float32(len(rows)) * lineH}
	}
	mark := func(ch rune, x, y float32, col ui.Color, alpha float64) {
		if alpha > 0.02 {
			p.Glyphs(s.scene.shapeRune(ch), x-adv/2, y+asc*0.5, col.Alpha(float32(math.Min(1, alpha))))
		}
	}
	word := func(text string, cx, y float32, col ui.Color, alpha float64) {
		rs := []rune(text)
		x := cx - float32(len(rs))*adv/2 + adv/2
		for _, ch := range rs {
			if ch != ' ' {
				mark(ch, x, y, col, alpha)
			}
			x += adv
		}
	}
	// wake leaves a short run of brighter water glyphs on the surface behind something that moves.
	wake := func(xNorm float64, dir float64, alpha float64) {
		for k := 1; k <= 5; k++ {
			u := xNorm - dir*float64(k)*0.022
			if u < 0.01 || u > 0.99 {
				continue
			}
			ch := '~'
			if k%2 == 0 {
				ch = '≈'
			}
			mark(ch, r.X+float32(u)*r.W, frame.surfaceY(r, u)-lineH*0.2, foamCol, alpha*(1.0-float64(k)/6.5))
		}
	}
	ea := edgeAlpha
	bumpU, bumpAmt := 0.0, 0.0

	var lastBounds ui.Rect

	switch act.kind {
	case EncounterAngler:
		// A small boat at rest: one figure, a rod, a lantern at the stern, a line down to a float.
		xNorm := 0.20 + 0.22*prog
		wy, sx, sl := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + sx
		boat := lines(hullCol,
			"     o    ,",
			"    /|\\  /",
			" ___/_\\_/___",
			" \\_________/",
		)
		b := sprite(boat, cx, wy+lineH*0.9, sl, ea, noClip)
		mark('*', b.X+adv*1.5, b.Y+lineH*1.6, lanternCol, ea*(0.75+0.25*math.Sin(tt*3.1)))
		// The line hangs from the rod tip to a float bobbing on the surface a little way off.
		tipX, tipY := b.X+adv*10.5, b.Y+lineH*0.3
		floatU := xNorm + float64((b.W*0.5+adv*3.5)/r.W)
		floatX := r.X + float32(floatU)*r.W
		floatY := frame.surfaceY(r, floatU) - lineH*0.1
		for k := 1; k <= 4; k++ {
			f := float32(k) / 5
			mark('\'', tipX+(floatX-tipX)*f, tipY+(floatY-tipY)*f*f, hullCol, ea*0.7)
		}
		bob := 0.0
		if reacting {
			bob = 5 * math.Sin(reactAge*14) * math.Exp(-reactAge*2)
		}
		mark('o', floatX, floatY+float32(bob), accentCol, ea)
		if reacting && reactAge < 1.3 {
			// A bite: a fish arcs out of the water by the float.
			t := reactAge / 1.3
			fish := "><>"
			if t > 0.5 {
				fish = "<><"
			}
			word(fish, floatX+float32(18*math.Sin(t*math.Pi)), floatY-float32(38*math.Sin(t*math.Pi)), accentCol, ea)
		}
		lastBounds = ui.Rect{X: b.X, Y: b.Y, W: b.W + adv*5, H: b.H}

	case EncounterPirate:
		// A three-master crossing the whole sea, pennants streaming, leaving a wake.
		xNorm := 0.5 - act.dir*0.68 + act.dir*1.36*prog
		wy, sx, sl := waveSample(math.Max(0.01, math.Min(0.99, xNorm)))
		cx := r.X + float32(xNorm)*r.W + sx
		flag := "~"
		if int(tt*3)%2 == 0 {
			flag = "≈"
		}
		sails := lines(sailCol,
			"       |"+flag,
			"      /|\\    |"+flag,
			"     /_|_\\  /|\\",
			"    /__|__\\/_|_\\",
		)
		hull := lines(hullCol,
			" ___\\__|_____|__/__",
			" \\   o    o    o  /",
			"  \\______________/",
		)
		ship := append(sails, hull...)
		if act.dir < 0 {
			ship = mirror(ship)
		}
		b := sprite(ship, cx, wy+lineH*1.1, sl*0.8, ea, noClip)
		if xNorm > 0.03 && xNorm < 0.97 {
			wake(xNorm-act.dir*0.085, act.dir, ea*0.9)
		}
		if reacting && reactAge < 0.9 {
			// A warning shot: a puff from the gun deck.
			t := reactAge / 0.9
			mark('*', cx+float32(act.dir)*(b.W*0.5+float32(t)*46), wy-lineH*0.6-float32(10*math.Sin(t*math.Pi)), lanternCol, ea*(1-t))
		}
		lastBounds = b

	case EncounterWhale:
		// A blue whale surfaces, breathes, rolls under and shows its flukes. The water lifts over it.
		xNorm := 0.5 + act.dir*(0.5-prog)*0.42
		wy, sx, sl := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + sx
		clip := wy + lineH*1.3
		up := smoothstep01(prog/0.13) * smoothstep01((0.66-prog)/0.10) // body above water
		if up > 0.01 {
			body := lines(whaleBack,
				"      .-\"\"\"\"\"\"-.",
				"   .-'          `-.   __",
				"  /  o             `-'  )",
				"  `-.________________.-'",
			)
			if act.dir > 0 {
				body = mirror(body)
			}
			sink := float32(1-up) * lineH * 4.4
			b := sprite(body, cx, wy+lineH*1.5+sink, sl*0.5, ea, clip)
			lastBounds = b
			// The blow: a short fountain over the blowhole, every few seconds and whenever it is greeted.
			puff := math.Mod(prog*float64(act.duration.Seconds()), 3.4) / 1.1
			if reacting {
				puff = reactAge / 1.1
			}
			if up > 0.85 && puff >= 0 && puff < 1 {
				hx := cx - float32(act.dir)*adv*2.5
				hy := b.Y - lineH*0.2
				pa := ea * math.Sin(puff*math.Pi)
				word("\\|/", hx, hy-float32(puff)*6, foamCol, pa)
				word("' '", hx, hy-lineH-float32(puff)*12, foamCol, pa*0.8)
				if reacting {
					mark('♪', hx+adv*3.5, hy-lineH*1.6-float32(puff)*16, accentCol, pa)
				}
			}
			bumpU, bumpAmt = xNorm, up
		} else if prog >= 0.70 {
			// Flukes: up, held, and down again.
			t := (prog - 0.70) / 0.30
			rise := smoothstep01(t/0.25) * smoothstep01((1-t)/0.30)
			tail := lines(whaleBack,
				" __     __",
				" \\ `-.-' /",
				"  `.   .'",
				"    | |",
				"    | |",
			)
			b := sprite(tail, cx, wy+lineH*1.2+float32(1-rise)*lineH*5.2, 0, ea, clip)
			lastBounds = b
			bumpU, bumpAmt = xNorm, 0.5*rise
			if t > 0.72 {
				// the splash it leaves
				ring := (t - 0.72) / 0.28
				word("~ ≈ ~", cx, wy-lineH*0.2, foamCol, ea*(1-ring))
			}
		}

	case EncounterDolphin:
		// Three dolphins travel across, each leaping in its own time.
		dolphin := lines(accentCol,
			"    _.-._",
			" ,-'   o `>",
			" `~--.__.-'",
		)
		if act.dir < 0 {
			dolphin = mirror(dolphin)
		}
		var hit ui.Rect
		for k := 0; k < 3; k++ {
			xNorm := 0.5 - act.dir*0.56 + act.dir*1.12*prog - act.dir*float64(k)*0.085
			if xNorm < 0.02 || xNorm > 0.98 {
				continue
			}
			cyc := math.Mod(prog*5.2+float64(k)*0.37, 1.0)
			if reacting {
				cyc = math.Mod(reactAge/1.1+float64(k)*0.2, 1.0) * 0.6
			}
			wy, _, _ := waveSample(xNorm)
			cx := r.X + float32(xNorm)*r.W
			if cyc < 0.6 {
				jt := cyc / 0.6
				h := float32(math.Sin(jt*math.Pi)) * lineH * 3.4
				tilt := -math.Cos(jt*math.Pi) * 0.16 * act.dir
				b := sprite(dolphin, cx, wy+lineH*1.1-h, -tilt, ea, wy+lineH*0.5)
				if hit.W == 0 {
					hit = b
				}
			} else if cyc < 0.78 {
				// where it went in
				mark('∘', cx+float32(act.dir)*adv*3, wy-lineH*0.2, foamCol, ea*(1-(cyc-0.6)/0.18))
			}
		}
		if hit.W == 0 {
			wy, _, _ := waveSample(0.5)
			hit = ui.Rect{X: r.X + r.W*0.25, Y: wy - lineH*4, W: r.W * 0.5, H: lineH * 5}
		}
		lastBounds = hit

	case EncounterLighthouse:
		// A lighthouse on its rock, lamp turning: the beam is written in dashes that reach out,
		// shorten and swing to the other side.
		xNorm := 0.14
		wy, _, _ := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W
		tower := append(lines(accentCol,
			"    /^\\",
			"   [   ]",
		), lines(hullCol,
			"    |=|",
			"    | |",
			"    |=|",
			"   /| |\\",
			" _/_|_|_\\_",
			"/_________\\",
		)...)
		b := sprite(tower, cx, wy+lineH*1.6, 0, ea, noClip)
		lampX, lampY := b.X+adv*5, b.Y+lineH*1.5
		turn := tt * 0.9
		if reacting {
			turn = tt*0.9 + reactAge*5
		}
		swing := math.Cos(turn) // +1 beam to the right, -1 to the left, 0 pointing at or away from us
		mark('*', lampX, lampY, lanternCol, ea*(0.6+0.4*math.Abs(swing)))
		reach := int(math.Abs(swing) * float64(r.W) * 0.62 / float64(adv))
		side := float32(1)
		if swing < 0 {
			side = -1
		}
		for k := 3; k < reach; k++ {
			f := float64(k) / math.Max(1, float64(reach))
			// the beam thins out with distance: = = - - -  -   -
			if (f > 0.45 && k%2 == 1) || (f > 0.75 && k%4 != 0) {
				continue
			}
			ch := '-'
			if f < 0.25 {
				ch = '='
			}
			x := lampX + side*float32(k)*adv
			if x < r.X+2 || x > r.X+r.W-2 {
				break
			}
			mark(ch, x, lampY+float32(f*f)*lineH*1.2, lanternCol, ea*(1-f)*0.95)
		}
		lastBounds = b

	case EncounterAurora:
		// Curtains of light hung in the night sky: columns of strokes whose length and brightness
		// ripple slowly from one end to the other.
		horizon := frame.surfaceY(r, 0.5)
		top := r.Y + 78 // below the page header
		room := horizon - top - lineH*2
		if room < lineH*4 {
			break
		}
		maxRows := int(math.Min(9, float64(room/lineH)))
		span := r.W * 0.76
		startX := r.X + r.W*0.12
		cols := int(span / adv)
		boost := 1.0
		if reacting {
			boost = 1.0 + 0.5*math.Exp(-reactAge*1.4)
		}
		for ci := 0; ci < cols; ci++ {
			u := float64(ci) / float64(max(1, cols-1))
			bell := math.Sin(u * math.Pi)
			fold := 0.5 + 0.5*math.Sin(u*9.0+tt*0.35)
			shimmer := 0.55 + 0.45*math.Sin(u*23.0-tt*1.1+float64(ci%3))
			rows := 2 + int(float64(maxRows-2)*bell*(0.35+0.65*fold))
			if ci%2 == 1 && fold < 0.35 {
				continue // folds of the curtain leave gaps
			}
			col := auroraA.MixOklab(auroraB, float32(math.Min(1, u*1.6)))
			if u > 0.6 {
				col = auroraB.MixOklab(auroraC, float32((u-0.6)/0.4))
			}
			x := startX + float32(ci)*adv
			y0 := top + float32(math.Sin(u*5.0+tt*0.28))*lineH*0.9 + float32(1-bell)*lineH
			for k := 0; k < rows; k++ {
				f := float64(k) / float64(max(1, rows-1))
				ch := '|'
				if k == 0 {
					ch = '\''
				} else if f > 0.8 {
					ch = ':'
				}
				al := ea * boost * (0.25 + 0.75*bell) * shimmer * (0.50 + 0.50*math.Sin(math.Min(1, f*1.15)*math.Pi)) * 1.25
				mark(ch, x, y0+float32(k)*lineH, col, al)
			}
		}
		lastBounds = ui.Rect{X: startX, Y: top - lineH, W: span, H: float32(maxRows+2) * lineH}
	}

	// What stands in the water moves it: the sea lifts over a whale on the next frame.
	s.scene.bumpU, s.scene.bumpAmt = bumpU, bumpAmt

	// Store latest hit bounds on the live encounter so clicks trigger visual + audio reactions cleanly
	if s.scene.active != nil && s.scene.active.kind == act.kind {
		s.scene.active.bounds = lastBounds
	}

	s.mu.Unlock()
	return true
}

// drawSceneWeather renders wave-coupled angled vector rain streaks + oval wave splashes,
// real branching vector storm lightning, soft drifting snow orbs, and layered volumetric coastal fog.
func (s *SeaState) drawSceneWeather(p *ui.Painter, r ui.Rect, v SeaView, wl float64, tt float64) bool {
	if v.Weather == "" || v.Weather == "clear" || v.HighContrast {
		return false
	}
	animate := v.Motion != "off" && !v.ReduceMotion
	px := 11
	if r.W >= 540 {
		px = 12
	}

	s.mu.Lock()
	s.scene.ensureFont(px)
	asc := s.scene.glyphAscent

	rainCol := ui.Hex("#1877E0")
	splashCol := ui.Hex("#4DABFF")
	snowCol := ui.Hex("#3B9BFF")
	fogCol := ui.Hex("#8BC9FF")
	boltGlow := ui.Hex("#3B9BFF")
	boltCore := ui.Hex("#EBF6FF")
	if v.Dark {
		rainCol = ui.Hex("#6CB5FF")
		splashCol = ui.Hex("#9CD0FF")
		snowCol = ui.Hex("#D9EEFF")
		fogCol = ui.Hex("#4E93DC")
		boltGlow = ui.Hex("#66B5FF")
		boltCore = ui.Hex("#FFFFFF")
	}

	adv := s.scene.glyphAdvance
	lineH := float32(px) * 1.05
	glyph := func(ch rune, x, y float32, col ui.Color) {
		p.Glyphs(s.scene.shapeRune(ch), x-adv/2, y+asc*0.5, col)
	}

	switch v.Weather {
	case "rain", "storm":
		// Rain is falling characters: a short tick in rain, a driven slash in a storm. Where a drop
		// meets the water it leaves a small ring that opens and fades on the surface.
		count := 30
		if r.W >= 540 {
			count = 48
		}
		drop, speed, drift := '\'', 1.40, 0.05
		if v.Weather == "storm" {
			count = int(float64(count) * 1.4)
			drop, speed, drift = '/', 2.05, 0.14
		}
		for i := 0; i < count; i++ {
			seedX := cellHash(i*17+3, 19)
			seedT := cellHash(i*29+7, 5)
			depth := 0.72 + 0.52*cellHash(i, 31)
			phase := math.Mod(seedT+tt*speed*depth, 1.0)
			xNorm := math.Mod(seedX-phase*drift+1.0, 1.0)
			x := r.X + float32(xNorm)*r.W
			surfY := s.frame.surfaceY(r, xNorm)
			span := float64(surfY - r.Y + 24)
			y := r.Y + float32(phase*span)
			alpha := float32((0.48 + 0.34*cellHash(i, 11)) * depth)
			if v.Weather == "storm" {
				alpha += 0.10
			}
			if alpha > 1 {
				alpha = 1
			}
			if y < surfY-4 {
				ch := drop
				if v.Weather == "rain" && depth > 1.02 {
					ch = '|' // the nearest drops fall as longer strokes
				}
				glyph(ch, x, y, rainCol.Alpha(alpha))
			} else if y < surfY+22 {
				f := (y - (surfY - 4)) / 26.0
				ring := '∘'
				if f > 0.45 {
					ring = 'o'
				}
				glyph(ring, x, surfY-2, splashCol.Alpha(alpha*(1.0-f)))
			}
		}

		// Lightning is a bolt written in / | \ from the sky to the sea. The flash itself is the
		// water flaring (see stormFlash in the sea), not a wash of light over the picture.
		if flash := stormFlash(v, tt); flash > 0 {
			seed := int(tt / stormStrikeEvery)
			bxNorm := 0.20 + 0.60*cellHash(seed, 71)
			endY := s.frame.surfaceY(r, bxNorm) - lineH
			x := r.X + float32(bxNorm)*r.W
			step := 0
			forkAt := 2 + int(cellHash(seed, 83)*3)
			for y := r.Y + lineH*0.5; y < endY; y += lineH {
				dir := int(cellHash(seed*13+step, 97)*3) - 1 // -1, 0, +1
				ch := '|'
				if dir < 0 {
					ch = '/'
				} else if dir > 0 {
					ch = '\\'
				}
				glyph(ch, x, y, boltCore.Alpha(float32(0.35+0.65*flash)))
				if step == forkAt || step == forkAt+3 {
					// a short side branch
					fdir := float32(1)
					fch := '\\'
					if step == forkAt {
						fdir, fch = -1, '/'
					}
					for k := 1; k <= 3; k++ {
						glyph(fch, x+fdir*adv*float32(k), y+lineH*float32(k), boltGlow.Alpha(float32(flash)*0.8))
					}
				}
				x += float32(dir) * adv
				step++
			}
		}

	case "snow":
		// Snow is slow asterisks, larger and smaller, swaying down and melting at the waterline.
		count := 26
		if r.W >= 540 {
			count = 42
		}
		flakes := []rune{'*', '∗', '*', '°'}
		for i := 0; i < count; i++ {
			seedX := cellHash(i*13+7, 23)
			seedT := cellHash(i*31+5, 17)
			spd := 0.20 + 0.16*cellHash(i, 41)
			phase := math.Mod(seedT+tt*spd, 1.0)
			sway := 0.028 * math.Sin(tt*1.2+float64(i)*1.9)
			xNorm := math.Mod(seedX+sway+1.0, 1.0)
			x := r.X + float32(xNorm)*r.W
			surfY := s.frame.surfaceY(r, xNorm)
			y := r.Y + float32(phase)*(surfY-r.Y+18)
			fade := 1.0
			if y > surfY-6 {
				fade = math.Max(0, 1.0-float64(y-(surfY-6))/24.0)
			}
			if alpha := float32((0.36 + 0.44*cellHash(i, 53)) * fade); alpha > 0.04 {
				glyph(flakes[i%len(flakes)], x, y, snowCol.Alpha(alpha))
			}
		}

	case "fog":
		// Fog is mostly told by the sea thinning out at the surface. Above it, a few long, faint
		// runs of dashes drift by: banks of mist, written rather than painted.
		for bi := 0; bi < 6; bi++ {
			spd := 0.012 + float64(bi%3)*0.007
			drift := math.Mod(tt*spd+float64(bi)*0.29, 1.5) - 0.25
			runLen := 14 + (bi%3)*7
			surfY := s.frame.surfaceY(r, math.Max(0.02, math.Min(0.98, drift)))
			y := surfY - lineH*(1.2+float32(bi%4)*1.5)
			if y < r.Y+lineH {
				continue
			}
			for k := 0; k < runLen; k++ {
				e := float64(k) / float64(runLen-1)
				env := math.Sin(e * math.Pi) // soft at both ends of the bank
				ch := '–'
				if (k+bi)%4 == 0 {
					ch = '~'
				}
				x := r.X + float32(drift)*r.W + (float32(k)-float32(runLen)/2)*adv*1.5
				if x < r.X-adv || x > r.X+r.W+adv {
					continue
				}
				glyph(ch, x, y, fogCol.Alpha(float32(0.50*env)))
			}
		}
	}
	s.mu.Unlock()
	return animate
}

// WeatherDisplayBadge builds a concise status badge for the Focus Clock header button.
func WeatherDisplayBadge(weatherMode, liveWeather string, liveTemp *float64) string {
	eff := weatherMode
	if eff == "live" {
		if liveWeather != "" {
			eff = liveWeather
		} else {
			eff = "clear"
		}
	}
	lbl := "晴空"
	switch eff {
	case "rain":
		lbl = "细雨"
	case "snow":
		lbl = "飞雪"
	case "storm":
		lbl = "雷雨"
	case "fog":
		lbl = "薄雾"
	}
	if weatherMode == "live" && liveTemp != nil {
		return strings.TrimSpace(lbl) + " " + formatTemp(*liveTemp)
	}
	return lbl
}

func formatTemp(c float64) string {
	return fmtIntTemp(int(math.Round(c)))
}

func fmtIntTemp(n int) string {
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	if n == 0 {
		digits = []byte{'0'}
	} else {
		for n > 0 {
			digits = append([]byte{byte('0' + n%10)}, digits...)
			n /= 10
		}
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits) + "°"
}
