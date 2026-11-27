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
	EncounterWhale      = "whale"      // 526Hz 深海蓝鲸
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

var AllEncounters = []EncounterMeta{
	{Kind: EncounterAngler, Name: "垂钓小船", Preview: `  o _/  \___/  <°)))><`, Hint: "全天候"},
	{Kind: EncounterPirate, Name: "海盗船", Preview: `|>[x] /__||__\ \_____/`, Hint: "全天候"},
	{Kind: EncounterWhale, Name: "526Hz 蓝鲸", Preview: `\|/  .-' o '-.  |\__/`, Hint: "专注中"},
	{Kind: EncounterDolphin, Name: "海豚", Preview: `  _.-._  / \  ~o~`, Hint: "高专注"},
	{Kind: EncounterLighthouse, Name: "灯塔", Preview: ` \*/ [|]  /  ✦`, Hint: "深夜"},
	{Kind: EncounterAurora, Name: "极光", Preview: `░▒~∿▒░ ~∿░▒~`, Hint: "连胜"},
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

func pickWeightedEncounter(sr *sceneRuntime, v SeaView, now time.Time) string {
	h := now.Local().Hour()
	lateNight := h >= 22 || h < 5
	highFocus := v.TodayMinutes >= 90 || v.TodayRounds >= 3

	wAngler := 26.0
	wPirate := 20.0
	wWhale := 24.0
	wDolphin := 12.0
	wLighthouse := 8.0
	wAurora := 8.0

	if v.Running {
		wWhale += 10
		wDolphin += 8
	}
	if lateNight {
		wLighthouse += 36
		wAurora += 14
	}
	if highFocus {
		wAurora += 20
		wDolphin += 14
		wWhale += 8
	}
	if v.Weather == "storm" {
		wPirate += 14
		wLighthouse += 10
	}

	total := wAngler + wPirate + wWhale + wDolphin + wLighthouse + wAurora
	r := sr.nextRand() * total
	switch {
	case r < wAngler:
		return EncounterAngler
	case r < wAngler+wPirate:
		return EncounterPirate
	case r < wAngler+wPirate+wWhale:
		return EncounterWhale
	case r < wAngler+wPirate+wWhale+wDolphin:
		return EncounterDolphin
	case r < wAngler+wPirate+wWhale+wDolphin+wLighthouse:
		return EncounterLighthouse
	default:
		return EncounterAurora
	}
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

// drawSceneBackdrop paints time-of-day sky lighting + volumetric clouds, glowing moon/sun,
// water reflections, twinkling stars, and smooth vector flapping seabirds behind the ocean waves.
func (s *SeaState) drawSceneBackdrop(p *ui.Painter, r ui.Rect, v SeaView, wl float64, tt float64, now time.Time) {
	if !v.TimeLighting || v.HighContrast {
		return
	}
	period, _ := DayPeriod(now)
	skyH := float32(math.Max(float64(r.H)*0.34, float64(r.H)*(wl+0.12)))
	if skyH > r.H {
		skyH = r.H
	}
	skyRect := ui.Rect{X: r.X, Y: r.Y, W: r.W, H: skyH}

	switch period {
	case "dawn":
		topCol := ui.RGBA(147, 211, 255, 0.24)
		midCol := ui.RGBA(255, 224, 192, 0.16)
		if v.Dark {
			topCol = ui.RGBA(35, 74, 116, 0.34)
			midCol = ui.RGBA(112, 134, 178, 0.18)
		}
		p.FillGradient(skyRect, ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: topCol, At: 0.0},
				{Color: midCol, At: 0.58},
				{Color: ui.Transparent, At: 1.0},
			},
		}, 0)
	case "dusk":
		topCol := ui.RGBA(42, 92, 158, 0.22)
		midCol := ui.RGBA(236, 174, 142, 0.16)
		if v.Dark {
			topCol = ui.RGBA(24, 48, 84, 0.38)
			midCol = ui.RGBA(118, 122, 168, 0.18)
		}
		p.FillGradient(skyRect, ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: topCol, At: 0.0},
				{Color: midCol, At: 0.62},
				{Color: ui.Transparent, At: 1.0},
			},
		}, 0)
	case "night":
		topCol := ui.RGBA(10, 46, 94, 0.18)
		if v.Dark {
			topCol = ui.RGBA(6, 14, 26, 0.44)
		}
		p.FillGradient(skyRect, ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: topCol, At: 0.0},
				{Color: ui.Transparent, At: 1.0},
			},
		}, 0)
	default: // day
		topCol := ui.RGBA(186, 226, 255, 0.18)
		if v.Dark {
			topCol = ui.RGBA(29, 62, 102, 0.22)
		}
		p.FillGradient(skyRect, ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: topCol, At: 0.0},
				{Color: ui.Transparent, At: 1.0},
			},
		}, 0)
	}

	if v.Weather == "storm" {
		stormTop := ui.RGBA(15, 42, 76, 0.26)
		if v.Dark {
			stormTop = ui.RGBA(5, 11, 20, 0.48)
		}
		p.FillGradient(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H * 0.48}, ui.LinearGradient{
			From: stormTop, To: ui.Transparent, Angle: 180,
		}, 0)
		return
	}

	px := 11
	if r.W >= 540 {
		px = 12
	}
	s.mu.Lock()
	s.scene.ensureFont(px)
	adv := s.scene.glyphAdvance
	asc := s.scene.glyphAscent
	drawLine := func(x, y float32, text string, col ui.Color) {
		cx := x
		for _, ch := range text {
			if ch != ' ' {
				gs := s.scene.shapeRune(ch)
				p.Glyphs(gs, cx, y+asc, col)
			}
			cx += adv
		}
	}

	starCol := ui.Hex("#0B4F96")
	cloudCol := ui.Hex("#88C6FF")
	warmCol := ui.Hex("#F7C88B")
	birdCol := ui.Hex("#1A5A9E")
	if v.Dark {
		starCol = ui.Hex("#CBE6FF")
		cloudCol = ui.Hex("#4B8FD8")
		warmCol = ui.Hex("#FFE0A8")
		birdCol = ui.Hex("#93C8FF")
	}

	// Helper: soft translucent radial puff with pill radius & early zero-alpha falloff (zero rectangular clip edges)
	drawPuff := func(cx, cy, rx, ry float32, col ui.Color, alpha float32) {
		if alpha <= 0.005 {
			return
		}
		rad := ry
		if rx < rad {
			rad = rx
		}
		p.FillRadialGradient(ui.Rect{X: cx - rx, Y: cy - ry, W: rx * 2, H: ry * 2}, ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: col.Alpha(alpha), At: 0.0},
				{Color: col.Alpha(alpha * 0.36), At: 0.42},
				{Color: ui.Transparent, At: 0.76},
				{Color: ui.Transparent, At: 1.0},
			},
		}, rad)
	}

	switch period {
	case "night":
		starCount := 16
		if r.W >= 540 {
			starCount = 28
		}
		starGlyphs := []string{"·", "·", "·", "✦"}
		for i := 0; i < starCount; i++ {
			sx := r.X + float32(0.06+0.88*cellHash(i*13+5, 3))*r.W
			sy := r.Y + float32(0.06+0.34*cellHash(i*7+2, 11))*skyH
			twinkle := 0.25 + 0.52*(0.5+0.5*math.Sin(tt*1.8+float64(i)*1.7))
			if v.Motion == "off" || v.ReduceMotion {
				twinkle = 0.48
			}
			g := starGlyphs[i%len(starGlyphs)]
			drawLine(sx, sy, g, starCol.Alpha(float32(twinkle)))
		}
		// Luminous glowing crescent moon + shimmering moonlight reflection dancing on live waves below!
		moonXNorm := 0.81
		mx := r.X + float32(moonXNorm)*r.W
		my := r.Y + float32(math.Max(46, float64(r.H)*0.13))
		moonCol := ui.Hex("#EBF5FF")
		if !v.Dark {
			moonCol = ui.Hex("#3B9BFF")
		}
		drawPuff(mx, my, 46, 46, moonCol, 0.14)
		drawPuff(mx, my, 22, 22, moonCol, 0.26)
		// Sculpted glowing crescent arc using smooth Bezier path
		var moon ui.Path
		moon.MoveTo(mx+4, my-12).
			QuadTo(mx-11, my-6, mx-11, my+1).
			QuadTo(mx-11, my+10, mx+5, my+13).
			QuadTo(mx-4, my+6, mx-4, my+0).
			QuadTo(mx-4, my-6, mx+4, my-12).Close()
		p.FillPath(&moon, moonCol.Alpha(0.86))

		// Shimmering vertical moonlight reflection ripples along the rolling water column below the moon
		waveY := s.frame.surfaceY(r, moonXNorm)
		for ri := 0; ri < 5; ri++ {
			rf := float32(ri)
			ry := waveY + 10 + rf*16
			if ry > r.Y+r.H-12 {
				break
			}
			wobble := float32(math.Sin(tt*2.0+float64(ri)*1.3) * 5.0)
			rw := float32(28 - ri*4)
			al := float32((0.20 - float64(ri)*0.032) * (0.75 + 0.25*math.Sin(tt*2.4+float64(ri))))
			drawPuff(mx+wobble, ry, rw, 4.5, moonCol, al)
		}

	case "dawn":
		sunXNorm := 0.77
		sx := r.X + float32(sunXNorm)*r.W
		sy := r.Y + float32(math.Max(46, float64(r.H)*0.14))
		drawPuff(sx, sy, 62, 54, warmCol, 0.22)
		drawPuff(sx, sy, 26, 26, warmCol, 0.36)
		var sun ui.Path
		sun.Circle(sx, sy, 11)
		p.FillPath(&sun, warmCol.Alpha(0.82))
		// Gentle horizon sun shimmer on the live waves
		waveY := s.frame.surfaceY(r, sunXNorm)
		for ri := 0; ri < 4; ri++ {
			ry := waveY + 10 + float32(ri)*15
			wobble := float32(math.Sin(tt*1.8+float64(ri)*1.4) * 6.0)
			drawPuff(sx+wobble, ry, float32(34-ri*5), 5.0, warmCol, float32(0.18-float64(ri)*0.035))
		}

	case "day":
		// Soft volumetric drifting cloud pillows (zero clunky ASCII boxes!)
		cloudCount := 2
		if r.W >= 540 {
			cloudCount = 3
		}
		for i := 0; i < cloudCount; i++ {
			spd := 0.012 + float64(i%2)*0.005
			drift := math.Mod(float64(i)*0.36+tt*spd+0.14, 1.30) - 0.15
			cx := r.X + float32(drift)*r.W
			cy := r.Y + float32(0.09+float64(i%2)*0.08+0.01*math.Sin(tt*0.5+float64(i)))*r.H
			sc := float32(0.85 + float64(i%2)*0.28)
			al := float32(0.24)
			if v.Dark {
				al = 0.16
			}
			drawPuff(cx-24*sc, cy+3*sc, 32*sc, 13*sc, cloudCol, al*0.85)
			drawPuff(cx+4*sc, cy-4*sc, 40*sc, 17*sc, cloudCol, al)
			drawPuff(cx+30*sc, cy+4*sc, 28*sc, 12*sc, cloudCol, al*0.80)
		}

	case "dusk":
		// Graceful soaring vector seabirds with continuous 60fps curved Bezier wingbeats!
		birdCount := 3
		if r.W >= 540 {
			birdCount = 5
		}
		for i := 0; i < birdCount; i++ {
			spd := 0.024 + float64(i%3)*0.006
			drift := math.Mod(float64(i)*0.22+tt*spd+0.12, 1.22) - 0.11
			bx := r.X + float32(drift)*r.W
			flap := math.Sin(tt*4.2 + float64(i)*1.7)
			by := r.Y + float32(0.09+float64(i%3)*0.055+0.012*math.Sin(tt*1.6+float64(i)))*r.H
			span := float32(7.5 + float64(i%2)*3.0)
			lift := float32(flap * 4.2)
			var bird ui.Path
			bird.MoveTo(bx-span, by-lift).
				QuadTo(bx-span*0.42, by-3.5-lift*0.35, bx, by).
				QuadTo(bx+span*0.42, by-3.5-lift*0.35, bx+span, by-lift)
			al := float32(0.46 - float64(i%3)*0.08)
			p.StrokePath(&bird, 1.35, birdCol.Alpha(al))
		}
	}
	s.mu.Unlock()
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
			dur := encounterDuration(kind)
			dir := 1.0
			if (kind == EncounterPirate || kind == EncounterWhale) && s.scene.nextRand() < 0.45 {
				dir = -1.0
			}
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
	if s.scene.active != nil {
		cp := *s.scene.active
		act = &cp
	}
	s.mu.Unlock()

	if spawnedNatural != "" && v.OnEncounter != nil {
		v.OnEncounter(spawnedNatural)
	}
	if act == nil {
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

	// Curated multi-tone palette strictly aligned with 526Hz Paper White x Ocean Blue / Deep Sea Night x Ice Blue
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
		// Clamp tilt so boats and creatures rock gracefully without distortion
		if slope > 0.26 {
			slope = 0.26
		} else if slope < -0.26 {
			slope = -0.26
		}
		surgeX = float32(-slope * 22) // a little horizontal surge down the face of the wave
		return surfY, surgeX, slope
	}

	// Center numeral readability protection: gently soften alpha if an encounter glides right across "25:00"
	numeralSoften := func(cx, cy float32) float64 {
		if compact {
			return 1.0
		}
		nuDx := math.Abs(float64(cx-(r.X+r.W*0.5)) / math.Max(1, float64(r.W)))
		nuDy := math.Abs(float64(cy-(r.Y+r.H*0.47)) / math.Max(1, float64(r.H)))
		if nuDx < 0.17 && nuDy < 0.14 {
			return 0.42
		}
		return 1.0
	}

	// drawTiltedSprite renders multi-line ASCII art with smooth rigid-body pitch tilt (slope)
	// so hulls, masts, and swimming silhouettes organically rock over every wave crest at 60fps.
	drawTiltedSprite := func(lines []string, cx, baseY float32, slope float64, col ui.Color, alphaScale float64) ui.Rect {
		if len(lines) == 0 || alphaScale <= 0.01 {
			return ui.Rect{}
		}
		maxCols := 0
		for _, ln := range lines {
			if n := len([]rune(ln)); n > maxCols {
				maxCols = n
			}
		}
		bw := float32(maxCols) * adv
		bh := float32(len(lines)) * lineH
		startX := cx - bw/2
		startY := baseY - bh
		alphaScale *= numeralSoften(cx, baseY-bh*0.5)
		finalCol := col.Alpha(float32(math.Min(1, alphaScale)))

		for li, ln := range lines {
			rowY := startY + float32(li)*lineH
			dyFromBase := float64(rowY - baseY) // negative above waterline
			x := startX
			for _, ch := range ln {
				if ch != ' ' {
					dx := float64(x + adv*0.5 - cx)
					// Small-angle rotation around (cx, baseY): x' = x - dy*slope, y' = y + dx*slope
					rx := x - float32(dyFromBase*slope*0.58)
					ry := rowY + float32(dx*slope) + asc
					p.Glyphs(s.scene.shapeRune(ch), rx, ry, finalCol)
				}
				x += adv
			}
		}
		return ui.Rect{X: startX, Y: startY, W: bw, H: bh}
	}

	// drawSpriteLine renders a single tilted line at (cx, y) with custom color.
	drawTiltedLine := func(ln string, cx, baseY, rowOffsetY float32, slope float64, col ui.Color, alphaScale float64) {
		if ln == "" || alphaScale <= 0.01 {
			return
		}
		n := len([]rune(ln))
		bw := float32(n) * adv
		startX := cx - bw/2
		rowY := baseY + rowOffsetY
		alphaScale *= numeralSoften(cx, rowY)
		finalCol := col.Alpha(float32(math.Min(1, alphaScale)))
		dyFromBase := float64(rowOffsetY)
		x := startX
		for _, ch := range ln {
			if ch != ' ' {
				dx := float64(x + adv*0.5 - cx)
				rx := x - float32(dyFromBase*slope*0.58)
				ry := rowY + float32(dx*slope) + asc
				p.Glyphs(s.scene.shapeRune(ch), rx, ry, finalCol)
			}
			x += adv
		}
	}

	// drawWaterWake paints a soft translucent reflection + undulating foam collar along the live wave line.
	drawWaterWake := func(xNorm float64, halfW float32, col ui.Color, alpha float64) {
		if alpha <= 0.01 {
			return
		}
		wy, sx, sl := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + sx
		// Soft translucent shadow/reflection pill beneath hull
		refl := ui.Rect{X: cx - halfW*0.95, Y: wy - 2, W: halfW * 1.9, H: 11}
		p.FillRadialGradient(refl, ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: col.Alpha(float32(0.20 * alpha)), At: 0.0},
				{Color: ui.Transparent, At: 1.0},
			},
		}, 6)
		// Subtle wave-following shimmer ripples
		drawTiltedLine("~  ·  ~  ·  ~", cx, wy+3, 0, sl*0.85, foamCol, 0.56*alpha)
		drawTiltedLine("·   ~   ·", cx, wy+11, 0, sl*0.6, foamCol, 0.30*alpha)
	}

	// drawGlow paints a smooth circular radial atmospheric glow with zero square boundary clipping.
	drawGlow := func(gx, gy, rad float32, col ui.Color, alpha float64) {
		if alpha <= 0.01 || rad <= 1 {
			return
		}
		rect := ui.Rect{X: gx - rad, Y: gy - rad, W: rad * 2, H: rad * 2}
		p.FillRadialGradient(rect, ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: col.Alpha(float32(alpha)), At: 0.0},
				{Color: col.Alpha(float32(alpha * 0.36)), At: 0.40},
				{Color: ui.Transparent, At: 0.76},
				{Color: ui.Transparent, At: 1.0},
			},
		}, rad)
	}

	var lastBounds ui.Rect

	switch act.kind {
	case EncounterAngler:
		// Serene coastal fishing skiff drifting smoothly with the swell, warm bow lantern, swaying silk line & leaping fish.
		var xNorm float64
		switch {
		case prog < 0.22:
			t := prog / 0.22
			ease := 1 - (1-t)*(1-t)*(1-t)
			xNorm = -0.06 + ease*0.29
		case prog < 0.80:
			t := (prog - 0.22) / 0.58
			xNorm = 0.23 + t*0.14 + 0.012*math.Sin(tt*0.75)
		default:
			t := (prog - 0.80) / 0.20
			xNorm = 0.37 + t*t*0.74
		}
		baseY, surgeX, slope := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + surgeX

		// Shimmering water reflection + soft bow lantern glow
		drawWaterWake(xNorm, adv*9.5, hullMid, edgeAlpha)
		lanternX := cx - adv*5.8 - float32(float64(-lineH*1.6)*slope*0.58)
		lanternY := baseY - lineH*1.4 + float32(float64(-adv*5.8)*slope)
		lanternPulse := 0.82 + 0.18*math.Sin(tt*3.4)
		drawGlow(lanternX, lanternY, 20, lanternCol, 0.30*lanternPulse*edgeAlpha)

		// Layered multi-tone skiff & angler silhouette (rocking smoothly with wave slope!)
		// Top hat + bent rod arc (accent/mid tone)
		drawTiltedLine(`      _        _.--'`, cx, baseY, -lineH*4.8, slope, hullMid, 0.90*edgeAlpha)
		drawTiltedLine(`     (_)    .-'     `, cx, baseY, -lineH*3.8, slope, hullCol, 0.94*edgeAlpha)
		drawTiltedLine(`     /|\  .'        `, cx, baseY, -lineH*2.8, slope, hullCol, 0.94*edgeAlpha)
		// Bow lantern glint + cabin seat
		drawTiltedLine(`  ✦  / \ /          `, cx, baseY, -lineH*1.8, slope, lanternCol, 0.92*edgeAlpha)
		// Deep navy carved skiff hull
		hullLines := []string{
			` .__/___\_________. `,
			`  \_______________/ `,
		}
		hullRect := drawTiltedSprite(hullLines, cx, baseY+lineH*0.2, slope, hullCol, 0.95*edgeAlpha)
		lastBounds = ui.Rect{X: hullRect.X, Y: baseY - lineH*5.2, W: hullRect.W, H: lineH * 5.6}

		// Curved swaying fishing line anchored accurately to the tilted rod tip
		rodDx := float64(adv * 9.0)
		rodDy := float64(-lineH * 4.4)
		rodTipX := cx + float32(rodDx) - float32(rodDy*slope*0.58)
		rodTipY := baseY + float32(rodDy) + float32(rodDx*slope)
		floatXNorm := math.Max(0.02, math.Min(0.98, float64(rodTipX-r.X)/math.Max(1, float64(r.W))))
		waterAtFloatY, _, floatSlope := waveSample(floatXNorm)

		bitePhase := math.Mod(prog*2.8, 1.0)
		biting := (prog >= 0.22 && prog <= 0.80 && bitePhase > 0.60 && bitePhase < 0.88) || reacting

		// Draw smooth dotted fishing line down to the live wave surface
		steps := max(3, int(math.Round(float64(waterAtFloatY-rodTipY)/float64(lineH*0.72))))
		if steps > 8 {
			steps = 8
		}
		for k := 1; k <= steps; k++ {
			kf := float64(k) / float64(steps)
			sway := float32(math.Sin(tt*2.4+kf*2.6) * 2.4 * math.Sin(kf*math.Pi))
			lx := rodTipX + sway
			ly := rodTipY + float32(kf)*float32(waterAtFloatY-rodTipY)
			ch := '·'
			if k%2 == 1 {
				ch = '│'
			}
			p.Glyphs(s.scene.shapeRune(ch), lx, ly+asc*0.5, accentCol.Alpha(float32(0.62*edgeAlpha)))
		}

		// Bobber & leaping fish reaction synced to the live wave surface
		if biting {
			drawTiltedLine("!", cx-adv*3.2, baseY, -lineH*5.6, slope, accentCol, 0.96*edgeAlpha)
			jumpT := 0.5
			if reacting {
				jumpT = math.Min(1, reactAge/1.15)
			} else {
				jumpT = (bitePhase - 0.60) / 0.28
			}
			arcY := float32(math.Sin(jumpT*math.Pi)) * lineH * 2.4
			arcX := float32((jumpT - 0.5) * float64(adv*5.0))
			fishSlope := -(math.Cos(jumpT * math.Pi)) * 0.35
			drawTiltedLine("<°)))><", rodTipX+arcX+adv*1.2, waterAtFloatY-arcY-lineH*0.3, 0, fishSlope, accentCol, 0.96*edgeAlpha)
			// Expanding splash ring right on the wave surface
			ringR := float32(8 + jumpT*22)
			ringAlpha := float32((1.0 - jumpT) * 0.55 * edgeAlpha)
			if ringAlpha > 0.02 {
				p.Stroke(ui.Rect{X: rodTipX - ringR, Y: waterAtFloatY - 4, W: ringR * 2, H: 8}, foamCol.Alpha(ringAlpha), 4, 1.2)
			}
			drawTiltedLine("° ~ · ~ °", rodTipX, waterAtFloatY+2, 0, floatSlope, foamCol, 0.80*edgeAlpha)
		} else {
			drawGlow(rodTipX, waterAtFloatY, 8, accentCol, 0.28*edgeAlpha)
			drawTiltedLine("~·~", rodTipX, waterAtFloatY+2, 0, floatSlope, foamCol, 0.66*edgeAlpha)
		}

	case EncounterPirate:
		// Grand tall-masted schooner riding every wave crest with billowing multi-tone sails, fluttering pennant & curved wake!
		xNorm := -0.10 + prog*1.20
		if act.dir < 0 {
			xNorm = 1.10 - prog*1.20
		}
		baseY, surgeX, slope := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + surgeX

		// Soft luminous sail backdrop + shimmering water wake underneath
		drawWaterWake(xNorm, adv*13.0, hullMid, edgeAlpha)
		drawGlow(cx, baseY-lineH*4.2, adv*10.5, sailCol, 0.14*edgeAlpha)

		// Animated fluttering top pennant at 60fps
		flagWave := int(math.Floor(tt*4.5)) % 3
		flagR := []string{`|>~`, `|≈>`, `|~>`}[flagWave]
		flagL := []string{`~<|`, `<≈|`, `<~|`}[flagWave]

		var sailLines, hullLines []string
		if act.dir >= 0 {
			sailLines = []string{
				`         ` + flagR + `          `,
				`       .-/|   |\-.     `,
				`      /  ||   ||  \    `,
				`     /__ ||___|| __\   `,
				`    /____||___||____\  `,
			}
			hullLines = []string{
				`      _||___||___/     `,
				`\____/  ·  ·  ·  \___/>`,
				` \___________________/ `,
			}
		} else {
			sailLines = []string{
				`          ` + flagL + `         `,
				`     .-/|   |\-.       `,
				`    /  ||   ||  \      `,
				`   /__ ||___|| __\     `,
				`  /____||___||____\    `,
			}
			hullLines = []string{
				`     \___||___||_      `,
				`<\___/  ·  ·  ·  \____/`,
				` \___________________/ `,
			}
		}

		// Render billowing sails in crisp sea-blue, carved hull in deep navy — BOTH tilted seamlessly by live wave slope!
		drawTiltedSprite(sailLines, cx, baseY-lineH*2.6, slope, sailCol, 0.90*edgeAlpha)
		hullRect := drawTiltedSprite(hullLines, cx, baseY+lineH*0.2, slope, hullCol, 0.94*edgeAlpha)
		lastBounds = ui.Rect{X: hullRect.X, Y: baseY - lineH*7.8, W: hullRect.W, H: lineH * 8.2}

		// Click reaction: gentle lantern flash along portholes + celebratory gust
		if reacting {
			gust := math.Sin(math.Min(1, reactAge/1.6) * math.Pi)
			drawGlow(cx, baseY-lineH*1.2, 26, lanternCol, 0.36*gust)
		}

		// Curved trailing foamy wake following the ACTUAL wave height behind the stern!
		wakeStep := -0.045
		if act.dir < 0 {
			wakeStep = 0.045
		}
		for wk := 1; wk <= 4; wk++ {
			wxNorm := xNorm + wakeStep*float64(wk) - wakeStep*0.6
			if wxNorm < 0.01 || wxNorm > 0.99 {
				continue
			}
			wy, wsx, wsl := waveSample(wxNorm)
			wx := r.X + float32(wxNorm)*r.W + wsx
			wAlpha := (1.0 - float64(wk)*0.20) * 0.68 * edgeAlpha
			glyph := []string{"≈ ~ ·", "~ · ~", "· ~ ·", "·   ·"}[wk-1]
			drawTiltedLine(glyph, wx, wy+3, 0, wsl, foamCol, wAlpha)
		}

	case EncounterWhale:
		// Iconic 526Hz Blue Whale: sculpted two-tone giant breaching smoothly, riding the swell,
		// emitting luminous 526Hz concentric acoustic rings & physics mist fountain, then lifting its fluke into a dive!
		xNorm := 0.12 + prog*0.76
		if act.dir < 0 {
			xNorm = 0.88 - prog*0.76
		}
		baseY, surgeX, waveSlope := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + surgeX

		reactPulse := 0.0
		if reacting {
			reactPulse = math.Sin(math.Min(1, reactAge/1.2) * math.Pi)
		}

		if prog < 0.76 {
			// Smooth breach emergence (0..0.18) -> graceful wave-riding glide (0.18..0.60) -> arching nose-down dive (0.60..0.76)
			subT := prog / 0.76
			emerge := 1.0
			divePitch := 0.0
			if subT < 0.22 {
				u := subT / 0.22
				emerge = u * u * (3 - 2*u)
				divePitch = -0.22 * (1.0 - u) * act.dir // nose up on breach
			} else if subT > 0.76 {
				u := (subT - 0.76) / 0.24
				emerge = 1.0 - 0.55*u*u
				divePitch = 0.28 * u * act.dir // nose down into dive
			}
			totalSlope := waveSlope*0.85 + divePitch
			whaleBaseY := baseY + float32((1.0-emerge)*float64(lineH)*3.2) - float32(reactPulse)*lineH*0.9

			// Underwater luminous silhouette glow + foam collar right along the wave line
			drawWaterWake(xNorm, adv*15.5, whaleBack, emerge*edgeAlpha)
			drawGlow(cx, whaleBaseY-lineH*2.2, adv*14.0, whaleHigh, 0.16*emerge*edgeAlpha)

			// 526Hz concentric acoustic rings expanding gently during song/click!
			if (prog >= 0.22 && prog <= 0.66) || reacting {
				ringCycle := math.Mod(tt*0.55, 1.0)
				if reacting {
					ringCycle = math.Min(1, reactAge/1.8)
				}
				for ri := 0; ri < 2; ri++ {
					rc := math.Mod(ringCycle+float64(ri)*0.48, 1.0)
					radX := float32(24 + rc*86)
					radY := float32(10 + rc*34)
					ra := float32(math.Sin(rc*math.Pi) * (0.34 + 0.34*reactPulse) * edgeAlpha)
					if ra > 0.02 {
						p.Stroke(ui.Rect{X: cx - radX, Y: whaleBaseY - lineH*2.2 - radY, W: radX * 2, H: radY * 2}, whaleHigh.Alpha(ra), radY, 1.3)
					}
				}
			}

			// Sculpted two-tone whale silhouette (deep ocean dorsal crown + lighter belly & animated undulating tail!)
			tailFlip := math.Sin(tt*2.8) > 0
			var backLines, bellyLines []string
			if act.dir >= 0 {
				tTop := `|\  `
				tMid := `| \_`
				if tailFlip {
					tTop = ` \  `
					tMid = `/ \_`
				}
				backLines = []string{
					`          _.-------._       `,
					`      .-'             '-.   `,
					tTop + `.'                 o '. `,
				}
				bellyLines = []string{
					tMid + `____                   \`,
					`    '-.   ~~~~~~~   _____.' `,
					`        '---------'         `,
				}
			} else {
				tTop := `  /|`
				tMid := `_/ |`
				if tailFlip {
					tTop = `  / `
					tMid = `_/ \`
				}
				backLines = []string{
					`       _.-------._          `,
					`   .-'             '-.      `,
					` .' o                 '.` + tTop,
				}
				bellyLines = []string{
					`/                   ____` + tMid,
					` '._____   ~~~~~~~   .-'    `,
					`         '---------'        `,
				}
			}
			drawTiltedSprite(backLines, cx, whaleBaseY-lineH*2.4, totalSlope, whaleBack, 0.94*emerge*edgeAlpha)
			bRect := drawTiltedSprite(bellyLines, cx, whaleBaseY+lineH*0.5, totalSlope, whaleMid, 0.90*emerge*edgeAlpha)
			lastBounds = ui.Rect{X: bRect.X, Y: whaleBaseY - lineH*5.6, W: bRect.W, H: lineH * 6.2}

			// Physics-based arched mist fountain spray from blowhole
			if (prog >= 0.18 && prog <= 0.68) || reacting {
				spT := math.Mod(tt*0.85, 1.0)
				if reacting {
					spT = math.Min(1, reactAge/1.4)
				}
				spAlpha := math.Sin(spT*math.Pi) * 0.92 * emerge * edgeAlpha
				holeOffX := adv * 5.2
				if act.dir < 0 {
					holeOffX = -adv * 5.2
				}
				holeX := cx + holeOffX - float32(float64(-lineH*4.8)*totalSlope*0.58)
				holeY := whaleBaseY - lineH*4.8 + float32(float64(holeOffX)*totalSlope)
				drawGlow(holeX, holeY-lineH*1.4, 22, whaleHigh, 0.24*spAlpha)

				// Rising and gracefully arcing fountain droplets (60fps ballistic curve!)
				for di := -2; di <= 2; di++ {
					spread := float64(di)
					dx := float32(spread * (4.5 + spT*11.0))
					// Parabolic arc: shoot up fast, curve outward & gently fall
					dy := -float32((32.0*spT - 18.0*spT*spT) * (1.0 - math.Abs(spread)*0.12))
					glyph := '·'
					if di == 0 && spT < 0.55 {
						glyph = '│'
					} else if math.Abs(spread) == 1 {
						glyph = '°'
					} else if reacting && spT > 0.3 {
						glyph = '✦'
					}
					p.Glyphs(s.scene.shapeRune(glyph), holeX+dx, holeY+dy+asc*0.5, whaleHigh.Alpha(float32(spAlpha)))
				}
			}
		} else {
			// Graceful fluke dive: tail lifts high out of the wave crest then slips cleanly beneath a glowing foam ring!
			tailT := (prog - 0.76) / 0.24
			lift := math.Sin(tailT * math.Pi)
			sinkY := baseY - float32(lift*float64(lineH)*1.2) + float32(tailT*tailT*float64(lineH)*2.4)
			tailLines := []string{
				`  |\     /|  `,
				`   \ \ / /   `,
				`    \   /    `,
				`     \ /     `,
			}
			tAlpha := (1.0 - tailT*0.78) * 0.94
			lastBounds = drawTiltedSprite(tailLines, cx, sinkY, waveSlope*0.6, whaleBack, tAlpha)
			// Lingering expanding foam ring on the live wave surface
			ringR := float32(16 + tailT*38)
			p.Stroke(ui.Rect{X: cx - ringR, Y: baseY - 5, W: ringR * 2, H: 10}, foamCol.Alpha(float32((1.0-tailT)*0.55)), 5, 1.3)
			drawTiltedLine("~ · ≈ · ~", cx, baseY+4, 0, waveSlope, foamCol, (1.0-tailT)*0.75)
		}

	case EncounterDolphin:
		// Graceful pod of 3 sleek dolphins leaping along true ballistic arcs with velocity-aligned pitch tilt
		// and synchronized live wave splashes at every exit & re-entry point!
		boost := 1.0
		if reacting {
			boost = 1.35
		}
		podOffsets := []struct {
			lag   float64
			scale float64
			col   ui.Color
			alpha float64
		}{
			{lag: 0.00, scale: 1.00, col: whaleBack, alpha: 0.96},
			{lag: 0.11, scale: 0.90, col: whaleMid, alpha: 0.88},
			{lag: 0.22, scale: 0.80, col: hullMid, alpha: 0.78},
		}

		for pi, member := range podOffsets {
			mp := prog - member.lag
			if mp < 0 || mp > 0.96 {
				continue
			}
			xNorm := 0.08 + mp*0.84
			if act.dir < 0 {
				xNorm = 0.92 - mp*0.84
			}
			wy, sx, wSlope := waveSample(xNorm)
			dx := r.X + float32(xNorm)*r.W + sx

			// 3 clean ballistic leaps across the horizon
			cycle := mp * 3.0
			leapIdx := math.Floor(cycle)
			leapPhase := cycle - leapIdx // [0, 1) within current jump
			arcH := math.Sin(leapPhase * math.Pi)
			// Analytical derivative of sin(leapPhase*pi) gives exact ballistic pitch angle!
			// Ascending -> nose UP, apex -> level, descending -> nose DOWN!
			velSlope := -math.Cos(leapPhase*math.Pi) * 0.34 * act.dir
			dy := wy - float32(arcH*boost*member.scale)*lineH*3.3

			var dolLines []string
			if act.dir >= 0 {
				dolLines = []string{
					`     _.-._     `,
					` |\.'    o'-.  `,
					`  \_____.--' \ `,
				}
			} else {
				dolLines = []string{
					`     _.-._     `,
					`  .-'o    './| `,
					` / '--._____/  `,
				}
			}

			// Fade gracefully as dolphin slips below the wave surface at leapPhase near 0 or 1
			surfVis := math.Min(1.0, arcH*3.4+0.15)
			b := drawTiltedSprite(dolLines, dx, dy+lineH*0.6, velSlope, member.col, member.alpha*surfVis*edgeAlpha)
			if pi == 0 {
				lastBounds = b
			}

			// Sparkling splash arcs right on the live wave surface at exit (0..0.22) and plunge (0.78..1.0)!
			if leapPhase < 0.24 || leapPhase > 0.76 {
				spFrac := leapPhase / 0.24
				if leapPhase > 0.76 {
					spFrac = (1.0 - leapPhase) / 0.24
				}
				spA := math.Sin(spFrac*math.Pi) * 0.78 * edgeAlpha
				ringR := float32(8 + (1.0-spFrac)*18)
				p.Stroke(ui.Rect{X: dx - ringR, Y: wy - 4, W: ringR * 2, H: 8}, foamCol.Alpha(float32(spA*0.65)), 4, 1.2)
				drawTiltedLine("° · ~ · °", dx, wy+2, 0, wSlope, foamCol, spA)
			}
		}

	case EncounterLighthouse:
		// Sculpted coastal headland lighthouse anchored on foaming rocks with real volumetric sweeping light cone & shooting stars!
		xNorm := 0.16
		if compact {
			xNorm = 0.50
		}
		wy, sx, wSlope := waveSample(xNorm)
		cx := r.X + float32(xNorm)*r.W + sx*0.25
		// Lighthouse sits firmly on rocky headland right along the swell (gentle reef spray at base, no silly rock tipping!)
		baseY := r.Y + float32(surfFrac)*r.H + (wy-(r.Y+float32(surfFrac)*r.H))*0.18

		lanternX := cx
		lanternY := baseY - lineH*5.4
		sweep := math.Sin(tt * 1.45) // smooth -1..+1 continuous sweep
		beamStrength := 0.55 + 0.45*math.Abs(sweep)
		if reacting {
			beamStrength = 1.0
		}

		// REAL VOLUMETRIC SWEEPING LIGHT BEAM (layered translucent horizontal-angled beam ribbons extending from lantern!)
		beamDir := float32(1.0)
		if sweep < 0 {
			beamDir = -1.0
		}
		beamLen := float32(math.Min(float64(r.W)*0.56, 360.0)) * float32(0.45+0.55*math.Abs(sweep))
		if compact {
			beamLen *= 0.55
		}
		beamCol := lanternCol
		if !v.Dark {
			beamCol = accentCol
		}
		// 28 finely overlapping soft light-cone slices for a silky, stair-step-free volumetric spotlight beam!
		const coneSteps = 28
		stepW := beamLen / coneSteps
		for si := 0; si < coneSteps; si++ {
			sf := float32(si) / float32(coneSteps)
			segX0 := lanternX + beamDir*(6+sf*beamLen)
			segW := stepW + 2.5
			segH := 8 + sf*64
			segY := lanternY - segH*0.5 - sf*18
			if beamDir < 0 {
				segX0 -= segW
			}
			// Smooth quadratic distance falloff
			falloff := (1.0 - float64(sf)) * (1.0 - float64(sf)*0.65)
			segAlpha := float32(falloff * 0.14 * beamStrength * edgeAlpha)
			if reacting {
				segAlpha *= 1.38
			}
			if segAlpha > 0.005 {
				p.FillGradient(ui.Rect{X: segX0, Y: segY, W: segW, H: segH}, ui.LinearGradient{
					Angle: 180,
					Stops: []ui.GradientStop{
						{Color: ui.Transparent, At: 0.0},
						{Color: beamCol.Alpha(segAlpha), At: 0.5},
						{Color: ui.Transparent, At: 1.0},
					},
				}, 0)
			}
		}

		// Warm glowing lantern core halo
		drawGlow(lanternX, lanternY, 26, beamCol, 0.44*beamStrength*edgeAlpha)
		drawGlow(lanternX, lanternY, 10, ui.Hex("#FFFFFF"), 0.55*beamStrength*edgeAlpha)

		// Sculpted coastal tower + striped body + rocky reef
		towerTop := []string{
			`      /\      `,
			`     [✦]      `,
			`    _| |_     `,
		}
		towerMid := []string{
			`    |===|     `,
			`   /|   |\    `,
			`  /_|===|_\   `,
		}
		reefBase := []string{
			` _/_______\_  `,
			`/___________\ `,
		}
		drawTiltedSprite(towerTop, cx, baseY-lineH*4.8, 0, accentCol, 0.94*edgeAlpha)
		drawTiltedSprite(towerMid, cx, baseY-lineH*1.8, 0, hullCol, 0.94*edgeAlpha)
		bRect := drawTiltedSprite(reefBase, cx, baseY+lineH*0.2, 0, hullMid, 0.90*edgeAlpha)
		lastBounds = ui.Rect{X: bRect.X, Y: baseY - lineH*7.8, W: bRect.W, H: lineH * 8.2}

		// Live ocean wave spray washing softly against the headland reef
		drawTiltedLine("~ · ≈ · ~ · ~", cx, wy+2, 0, wSlope*0.7, foamCol, 0.72*edgeAlpha)

		// Smooth 60fps diagonal shooting stars with fading comet tails across upper-right sky
		mCycle := math.Mod(prog*2.6, 1.0)
		if mCycle < 0.36 {
			mt := mCycle / 0.36
			mx := r.X + r.W*float32(0.86-mt*0.30)
			my := r.Y + r.H*float32(0.08+mt*0.18)
			mAlpha := math.Sin(mt*math.Pi) * 0.92 * edgeAlpha
			drawGlow(mx, my, 12, whaleHigh, 0.32*mAlpha)
			for ti := 0; ti < 5; ti++ {
				tf := float32(ti)
				tx := mx + tf*8.5
				ty := my - tf*5.0
				ta := float32(mAlpha * (1.0 - float64(ti)*0.20))
				ch := '·'
				if ti == 0 {
					ch = '✦'
				} else if ti == 1 {
					ch = '·'
				}
				p.Glyphs(s.scene.shapeRune(ch), tx, ty+asc*0.5, whaleHigh.Alpha(ta))
			}
		}

	case EncounterAurora:
		// Silky multi-curtain volumetric ocean-blue & aurora-teal light ribbons undulating smoothly at 60fps
		// across the upper sky, dusted with delicate high-altitude starlight shimmer (zero blocky ASCII clutter!).
		skyTop := r.Y + float32(math.Max(22, float64(r.H)*0.08))
		curtainH := float32(math.Min(135, math.Max(68, float64(r.H)*0.24)))
		if compact {
			skyTop = r.Y + float32(r.H)*0.16
			curtainH = float32(r.H) * 0.28
		}
		boost := 1.0
		if reacting {
			boost = 1.0 + 0.48*math.Sin(math.Min(1, reactAge/1.6)*math.Pi)
		}

		// 64 finely overlapping translucent vertical curtains with continuous color blending (zero stripe edges!)
		mixCol := func(a, b ui.Color, f float32) ui.Color {
			lerpU8 := func(x, y uint8) uint8 {
				return uint8(math.Round(float64(x) + (float64(y)-float64(x))*float64(f)))
			}
			return ui.Color{
				R: lerpU8(a.R, b.R),
				G: lerpU8(a.G, b.G),
				B: lerpU8(a.B, b.B),
				A: lerpU8(a.A, b.A),
			}
		}
		const bands = 64
		spanW := r.W * 0.80
		startX := r.X + (r.W-spanW)*0.5
		bandW := (spanW / float32(bands)) * 2.4
		for bi := 0; bi < bands; bi++ {
			u := float64(bi) / float64(bands-1)
			bell := math.Sin(u * math.Pi)
			bell = bell * bell // silky feathered horizontal taper
			w1 := math.Sin(u*4.8 - tt*0.85)
			w2 := math.Sin(u*8.4 + tt*0.52)
			waveY := float32((w1*0.62 + w2*0.38) * 18.0)
			hScale := float32(0.74 + 0.26*(0.5+0.5*math.Sin(u*6.2-tt*1.05)))

			// Smooth continuous color wave across ocean-blue -> aqua -> aurora-mint
			ct := float32(0.5 + 0.5*math.Sin(u*3.6+tt*0.45))
			var col ui.Color
			if ct < 0.5 {
				col = mixCol(auroraA, auroraB, ct*2)
			} else {
				col = mixCol(auroraB, auroraC, (ct-0.5)*2)
			}
			shimmer := 0.62 + 0.38*(0.5+0.5*math.Sin(u*9.0-tt*1.25))
			al := float32(math.Min(0.16, bell*0.115*edgeAlpha*boost*shimmer))
			if al > 0.004 {
				bx := startX + float32(u)*spanW - bandW*0.5
				by := skyTop + waveY
				bh := curtainH * hScale
				p.FillGradient(ui.Rect{X: bx, Y: by, W: bandW, H: bh}, ui.LinearGradient{
					Angle: 180,
					Stops: []ui.GradientStop{
						{Color: ui.Transparent, At: 0.0},
						{Color: col.Alpha(al * 0.55), At: 0.28},
						{Color: col.Alpha(al), At: 0.62},
						{Color: ui.Transparent, At: 1.0},
					},
				}, 0)
			}

			// Delicate drifting high-altitude starlight shimmer along the ribbon
			if bi%5 == 2 {
				sx := startX + float32(u)*spanW
				sy := skyTop + waveY + curtainH*0.52 + float32(math.Sin(u*11.0+tt*1.3)*8.0)
				sa := float32(math.Min(0.68, bell*0.58*edgeAlpha*boost))
				ch := '·'
				if (bi/5)%3 == 0 {
					ch = '~'
				} else if (bi/5)%4 == 1 {
					ch = '✦'
				}
				if sa > 0.04 {
					p.Glyphs(s.scene.shapeRune(ch), sx, sy+asc*0.5, col.Alpha(sa))
				}
			}
		}
		lastBounds = ui.Rect{
			X: startX,
			Y: skyTop - 10,
			W: spanW,
			H: curtainH + 20,
		}
	}

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
