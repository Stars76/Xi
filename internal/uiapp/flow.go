package uiapp

import (
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// Pure horizontal water-ripple character ramp (zero noisy brackets/circles/vertical glyphs).
var seaRunes = []string{" ", "·", "·", "~", "~", "≈", "≈", "≋"}

var (
	seaLightColors   = []ui.Color{ui.Hex("#eaf5ff"), ui.Hex("#c9e8ff"), ui.Hex("#93d3ff"), ui.Hex("#4dabff"), ui.Hex("#1877e0"), ui.Hex("#0b4f96"), ui.Hex("#062f5c")}
	seaLightHCColors = []ui.Color{ui.Hex("#f4faff"), ui.Hex("#dcefff"), ui.Hex("#b7e0ff"), ui.Hex("#7cc0ff"), ui.Hex("#3d9dff"), ui.Hex("#1a63c4"), ui.Hex("#0b3f7e")}
	seaDarkColors    = []ui.Color{ui.Hex("#18293D"), ui.Hex("#1D3754"), ui.Hex("#234A74"), ui.Hex("#2B629D"), ui.Hex("#3780CC"), ui.Hex("#52A1F0"), ui.Hex("#84C5FF")}
	seaDarkHCColors  = []ui.Color{ui.Hex("#1D324B"), ui.Hex("#254569"), ui.Hex("#2E5E91"), ui.Hex("#3B7BC0"), ui.Hex("#549DF0"), ui.Hex("#7BC0FF"), ui.Hex("#B3DCFF")}
)

// SeaState retains smooth water-level easing and glyph cache across frames.
type SeaState struct {
	mu          sync.Mutex
	levelShown  float64
	hasLevel    bool
	wavePhase   float64
	moodClock   float64 // seconds of animated time; drives the slow drift between sea moods
	moodCur     seaMood // the mood the water is in right now, eased towards what the weather asks for
	moodInit    bool
	frame       seaFrame // the surface as drawn this frame (read by weather, lighting and encounters)
	lastPaintAt time.Time
	cachedSize  int
	glyphs      [][]ui.Glyph
	ascents     []float32
	advances    []float32

	ribbonGlyphs   [][]ui.Glyph
	ribbonAscents  []float32
	ribbonAdvances []float32

	scene sceneRuntime
}

func NewSeaState() *SeaState {
	return &SeaState{
		levelShown: 1.0,
		wavePhase:  1.5,
	}
}

func (s *SeaState) ensureGlyphs(cell int) {
	px := max(8, int(math.Round(float64(cell)*0.92)))
	if s.cachedSize == px && len(s.glyphs) == len(seaRunes) {
		return
	}
	s.cachedSize = px
	f := ui.Font{Family: FontMono, Size: float32(px)}
	ascent := f.Metrics().Ascent
	s.glyphs = make([][]ui.Glyph, len(seaRunes))
	s.ascents = make([]float32, len(seaRunes))
	s.advances = make([]float32, len(seaRunes))
	for i, sym := range seaRunes {
		gs := ui.Shape(sym, f)
		var adv float32
		for _, g := range gs {
			adv += g.Advance
		}
		s.glyphs[i] = gs
		s.ascents[i] = ascent
		s.advances[i] = adv
	}
}

func cellHash(a, b int) float64 {
	x := uint32((a+1)*374761393 + (b+1)*668265263)
	x = (x ^ (x >> 13)) * 1274126177
	x = x ^ (x >> 16)
	return float64(x) / 4294967296.0
}

// The body of the sea is built from three glyphs only, light to heavy. Depth is carried by the
// weight of the glyph itself and its colour — never by a fill, a stroke or random gaps.
// The values index seaRunes: ~, ≈, ≋.
var seaBodyGlyphs = [3]int{3, 5, 7}

// seaSurfaceH is the wave height at which the first glyph appears. Nothing is drawn above it,
// so the top row of ~ is the waterline.
const seaSurfaceH = 0.42

// seaMood is one character of the same continuous wave: how wide (K), how tall (Amp), how fast (Speed).
type seaMood struct{ K, Amp, Speed float64 }

// The weather decides how the sea moves. Each weather has a small set of moods the water drifts
// between, so rain, storm, fog and snow are states of the sea itself and not effects laid over it.
var (
	moodFamiliar = seaMood{K: 4.2, Amp: 0.14, Speed: 1.0}   // the everyday swell
	moodCalm     = seaMood{K: 3.2, Amp: 0.09, Speed: 0.6}   // calm
	moodRollers  = seaMood{K: 2.6, Amp: 0.17, Speed: 0.85}  // long rollers
	moodFine     = seaMood{K: 5.6, Amp: 0.11, Speed: 1.15}  // fine, quicker waves
	moodChop     = seaMood{K: 6.4, Amp: 0.13, Speed: 1.3}   // rain-beaten chop
	moodStorm    = seaMood{K: 2.4, Amp: 0.22, Speed: 1.4}   // storm rollers
	moodSurge    = seaMood{K: 3.6, Amp: 0.19, Speed: 1.55}  // storm surge
	moodStill    = seaMood{K: 3.0, Amp: 0.06, Speed: 0.45}  // near-still water under fog
	moodHush     = seaMood{K: 3.0, Amp: 0.08, Speed: 0.5}   // slow, quiet snow sea
	seaMoodSets  = map[string][]seaMood{
		"clear": {moodFamiliar, moodCalm, moodRollers},
		"rain":  {moodFine, moodChop},
		"storm": {moodStorm, moodSurge},
		"fog":   {moodStill, moodCalm},
		"snow":  {moodHush, moodCalm},
	}
	// How often a broad swell crosses the surface, in seconds (0 = never).
	seaSwellEvery = map[string]float64{"clear": 240, "rain": 150, "storm": 70}
)

const (
	seaMoodHold  = 90.0 // seconds per mood (roughly half held, half gliding to the next)
	seaSwellPass = 26.0 // a swell takes this long to cross
	seaMoodEase  = 7.0  // seconds for the water to settle into a new mood, e.g. when the weather turns
)

func smoothstep01(x float64) float64 {
	x = math.Max(0, math.Min(1, x))
	return x * x * (3 - 2*x)
}

func seaWeatherKey(weather string) string {
	if _, ok := seaMoodSets[weather]; ok {
		return weather
	}
	return "clear"
}

// seaMoodFor blends the two moods the given weather is between at clock (seconds of animated time).
func seaMoodFor(weather string, clock float64) seaMood {
	set := seaMoodSets[seaWeatherKey(weather)]
	ph := clock / seaMoodHold
	i := int(math.Floor(ph))
	a, b := set[i%len(set)], set[(i+1)%len(set)]
	f := smoothstep01((ph - math.Floor(ph) - 0.45) / 0.55)
	return seaMood{a.K + (b.K-a.K)*f, a.Amp + (b.Amp-a.Amp)*f, a.Speed + (b.Speed-a.Speed)*f}
}

// seaSwellAt returns the centre (0..1 across the width, may be off-screen) and strength (0..1)
// of the occasional broad swell at clock.
func seaSwellAt(weather string, clock float64) (centre, strength float64) {
	every := seaSwellEvery[seaWeatherKey(weather)]
	if every <= 0 {
		return 0, 0
	}
	tIn := math.Mod(clock, every) - (every - seaSwellPass - 10)
	if tIn < 0 || tIn > seaSwellPass {
		return 0, 0
	}
	f := tIn / seaSwellPass
	return -0.3 + f*1.6, smoothstep01(f/0.25) * smoothstep01((1-f)/0.25)
}

// seaFrame is the water as drawn this frame. Weather, lighting and encounters read the waterline
// from it, so everything floats on, falls onto or rises out of the very same surface.
type seaFrame struct {
	wl       float64 // waterline as a fraction of the height (0 top .. 1 bottom), before waves
	tt       float64 // wave phase
	mood     seaMood
	swellU   float64
	swellAmt float64
}

// lift is how much the passing swell raises the wave height at xNorm.
func (f seaFrame) lift(xNorm float64) float64 {
	if f.swellAmt <= 0 {
		return 0
	}
	du := (xNorm - f.swellU) / 0.24
	return 0.086 * f.swellAmt * math.Exp(-du*du)
}

// height is the sea's wave-height field: glyphs are drawn wherever it reaches seaSurfaceH.
func (f seaFrame) height(xNorm, vv float64) float64 {
	return (vv-f.wl)*1.15 + 0.42 + 0.10*math.Sin(vv*3.4+f.tt*0.9) +
		f.mood.Amp*math.Sin(xNorm*f.mood.K+vv*1.5-f.tt*1.3) + f.lift(xNorm)
}

// surfaceVV solves height(xNorm, vv) == seaSurfaceH for vv: where the waterline is at xNorm.
func (f seaFrame) surfaceVV(xNorm float64) float64 {
	vv := f.wl
	for i := 0; i < 3; i++ {
		rest := 0.10*math.Sin(vv*3.4+f.tt*0.9) + f.mood.Amp*math.Sin(xNorm*f.mood.K+vv*1.5-f.tt*1.3) + f.lift(xNorm)
		vv = f.wl + (seaSurfaceH-0.42-rest)/1.15
	}
	return vv
}

// surfaceY is the waterline in pixels at xNorm inside r.
func (f seaFrame) surfaceY(r ui.Rect, xNorm float64) float32 {
	return r.Y + float32(f.surfaceVV(xNorm))*r.H
}

// surfaceSlope is the waterline's slope (dy/dx in pixels) at xNorm, for tilting things that float.
func (f seaFrame) surfaceSlope(r ui.Rect, xNorm float64) float64 {
	const d = 0.01
	dy := float64(f.surfaceVV(xNorm+d)-f.surfaceVV(xNorm-d)) * float64(r.H)
	return dy / (2 * d * math.Max(1, float64(r.W)))
}

type SeaView struct {
	RemainRatio  float64
	RemainSec    int
	Running      bool
	Paused       bool
	Idle         bool
	Motion       string
	HighContrast bool
	Dark         bool
	ReduceMotion bool
	PointerX     float32
	PointerY     float32
	PointerIn    bool

	// Atmospheric scene & random encounters (all optional; zero-value = classic clear ocean)
	Weather      string            // clear | rain | snow | storm | fog
	TimeLighting bool              // follow local time-of-day lighting
	EncountersOn bool              // allow random ocean easter eggs
	TodayMinutes int               // informs high-focus encounter weighting
	TodayRounds  int               // informs high-focus encounter weighting
	Now          time.Time         // wall-clock time for time-of-day & encounter timing
	OnEncounter  func(kind string) // called when a natural random encounter spawns
}

func (s *SeaState) Draw(c *ui.Context, box ui.Element, v SeaView) {
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		w := float64(r.W)
		h := float64(r.H)
		if w <= 1 || h <= 1 {
			return
		}
		remain := math.Max(0, math.Min(1, v.RemainRatio))
		// In idle state, leave gentle upper-sky breathing room instead of filling 100% to the top edge.
		if v.Idle && !v.Running && !v.Paused && remain > 0.78 {
			remain = 0.78
		}

		now := v.Now
		if now.IsZero() {
			now = time.Now()
		}
		hasWeatherAnim := v.Weather != "" && v.Weather != "clear" && v.Motion != "off" && !v.ReduceMotion
		hasActiveEnc := s.ActiveEncounterKind(now) != "" && v.Motion != "off" && !v.ReduceMotion
		// Keep a gentle living breath when idle (so waves, clouds, stars & birds never feel frozen);
		// only pause motion when explicitly paused, reduced-motion, or motion == "off".
		still := v.Motion == "off" || v.ReduceMotion || (v.Paused && !hasWeatherAnim && !hasActiveEnc)

		s.mu.Lock()
		if !s.hasLevel {
			s.levelShown = remain
			s.hasLevel = true
		} else {
			rate := 0.07
			if v.Running {
				rate = 0.12
			}
			s.levelShown += (remain - s.levelShown) * rate
			if math.Abs(remain-s.levelShown) < 0.0015 {
				s.levelShown = remain
			}
		}
		if !still {
			paintNow := p.Now()
			speed := 0.36 // calm living breath when idle
			if hasActiveEnc || hasWeatherAnim {
				speed = 0.62
			}
			if v.Running {
				if v.RemainSec > 0 && v.RemainSec <= 60 {
					speed = 2.1
				} else {
					speed = 1.0
				}
			}
			if v.Motion == "calm" {
				speed *= 0.52
			}
			if !s.lastPaintAt.IsZero() {
				dt := paintNow.Sub(s.lastPaintAt).Seconds()
				if dt > 0 {
					if dt > 0.08 {
						dt = 0.08
					}
					// The phase is integrated, so a mood's change of speed never makes the water jump.
					s.wavePhase += dt * speed * s.moodCur.Speed
					s.moodClock += dt
					// Ease into whatever the weather now asks of the sea.
					want := seaMoodFor(v.Weather, s.moodClock)
					k := 1 - math.Exp(-dt/seaMoodEase)
					s.moodCur.K += (want.K - s.moodCur.K) * k
					s.moodCur.Amp += (want.Amp - s.moodCur.Amp) * k
					s.moodCur.Speed += (want.Speed - s.moodCur.Speed) * k
				}
			}
			s.lastPaintAt = paintNow
		} else {
			s.lastPaintAt = time.Time{}
		}
		if !s.moodInit {
			s.moodCur = seaMoodFor(v.Weather, s.moodClock)
			s.moodInit = true
		}
		tt := s.wavePhase
		mood := s.moodCur
		swellU, swellAmt := seaSwellAt(v.Weather, s.moodClock)
		levelShown := s.levelShown
		s.frame = seaFrame{wl: 1.0 - levelShown, tt: tt, mood: mood, swellU: swellU, swellAmt: swellAmt}
		frame := s.frame

		compact := w < 460 || h < 480
		maxCells := 2300.0
		maxCell := 18.0
		if compact {
			maxCells = 820.0
			maxCell = 14.0
		}
		ideal := math.Sqrt((w * h) / maxCells)
		cell := int(math.Max(9, math.Min(maxCell, math.Round(ideal))))
		cols := max(1, int(math.Ceil(w/float64(cell))))
		rows := max(1, int(math.Ceil(h/float64(cell))))
		s.ensureGlyphs(cell)
		glyphs := s.glyphs
		ascents := s.ascents
		advances := s.advances
		s.mu.Unlock()

		colors := seaLightColors
		if v.Dark {
			if v.HighContrast {
				colors = seaDarkHCColors
			} else {
				colors = seaDarkColors
			}
		} else if v.HighContrast {
			colors = seaLightHCColors
		}

		wl := 1.0 - levelShown
		// Atmospheric sky lighting + stars/moon/clouds/birds behind the waves
		s.drawSceneBackdrop(p, r, v, wl, tt, now)

		cellF := float64(cell)
		half := cellF / 2.0
		hurryBoost := 0.0
		if v.Running && v.RemainSec > 0 && v.RemainSec <= 60 {
			hurryBoost = 0.08
		}
		pxLocal := float64(v.PointerX - r.X)
		pyLocal := float64(v.PointerY - r.Y)
		fog := v.Weather == "fog"
		flash := stormFlash(v, tt)
		flashStep := -2 // on paper, light is less ink
		if v.Dark {
			flashStep = 2
		}

		for row := 0; row < rows; row++ {
			y := float64(row)*cellF + half
			vv := y / h
			// Fast-skip empty rows well above the wave crest
			if vv < wl-0.58 {
				continue
			}
			rowBase := (vv-wl)*1.15 + 0.42 + 0.10*math.Sin(vv*3.4+tt*0.9)
			if rowBase+mood.Amp+0.30 < seaSurfaceH {
				continue // whole row is above the waterline
			}
			rowPhase := vv*1.5 - tt*1.3
			rowShift := math.Mod(float64(row)*0.37, 1.0) * cellF
			dy := y - pyLocal
			for col := -1; col <= cols; col++ {
				x := float64(col)*cellF + half + rowShift
				if x < -cellF*0.5 || x > w+cellF*0.5 {
					continue
				}
				uu := x / w
				waveSin := math.Sin(uu*mood.K + rowPhase)
				H := rowBase + mood.Amp*waveSin + frame.lift(uu) // the passing swell lifts the surface locally
				if v.PointerIn {
					dx := x - pxLocal
					distSq := dx*dx + dy*dy
					if distSq < 76*76 {
						dist := math.Sqrt(distSq)
						falloff := 1.0 - dist/76.0
						H += 0.22 * falloff * falloff * math.Sin(dist*0.18-tt*5.5)
					}
				}
				// Nothing above the waterline, and no gaps below it: every cell under the surface
				// carries a glyph, so the water is one continuous field of characters.
				if H < seaSurfaceH {
					continue
				}
				if H > 1 {
					H = 1
				}
				depth := (H - seaSurfaceH) / (1 - seaSurfaceH) // 0 at the waterline .. 1 in deep water
				idx := seaBodyGlyphs[min(2, int(depth*3))]
				ci := min(len(colors)-1, 2+int(math.Round(depth*float64(len(colors)-3))))
				alpha := 0.50 + 0.48*depth + hurryBoost
				if fog {
					// Fog is told by the water itself: the surface thins out into the mist.
					alpha *= 0.30 + 0.70*smoothstep01(depth/0.45)
				}
				if flash > 0 {
					// Lightning lights the water for an instant: the glyphs flare towards the light end of the ramp.
					ci = max(0, min(len(colors)-1, ci+flashStep))
					alpha = math.Min(1, alpha+0.22*flash)
				}
				if v.HighContrast {
					alpha = math.Min(1, alpha+0.22)
				}
				if alpha > 1 {
					alpha = 1
				}
				alpha = math.Round(alpha*16) / 16
				yOff := waveSin * cellF * 0.16
				gx := r.X + float32(x) - advances[idx]/2
				gy := r.Y + float32(y-half+yOff) + ascents[idx] + float32(cellF-float64(ascents[idx]))*0.18
				p.Glyphs(glyphs[idx], gx, gy, colors[ci].Alpha(float32(alpha)))
			}
		}

		// Task-completion ascending wave pulse
		pulseAnim := s.drawTaskPulse(p, r, v, now)
		// Surface & breaching random ASCII easter eggs (all 6 + click reaction quotes)
		encActive := s.drawSceneEncounters(p, r, v, wl, tt, now)
		// Weather particle layer (rain + splashes, snow, storm lightning, fog)
		weatherAnim := s.drawSceneWeather(p, r, v, wl, tt)

		if !still || encActive || weatherAnim || pulseAnim {
			if v.Motion == "calm" || (v.Idle && !encActive && !weatherAnim && !pulseAnim) {
				p.AnimationFrameRate(30)
			} else {
				p.AnimationFrameRate(60)
			}
		} else if math.Abs(remain-levelShown) >= 0.0015 {
			p.AnimationFrameRate(60)
		}
	})
}

// The Today tide ribbon uses the same three glyphs as the sea, at a smaller size.
var ribbonRunes = []string{"~", "≈", "≋"}

func (s *SeaState) ensureRibbonGlyphs() {
	if len(s.ribbonGlyphs) == len(ribbonRunes) {
		return
	}
	f := ui.Font{Family: FontMono, Size: 12}
	ascent := f.Metrics().Ascent
	s.ribbonGlyphs = make([][]ui.Glyph, len(ribbonRunes))
	s.ribbonAscents = make([]float32, len(ribbonRunes))
	s.ribbonAdvances = make([]float32, len(ribbonRunes))
	for i, sym := range ribbonRunes {
		gs := ui.Shape(sym, f)
		var adv float32
		for _, g := range gs {
			adv += g.Advance
		}
		s.ribbonGlyphs[i] = gs
		s.ribbonAscents[i] = ascent
		s.ribbonAdvances[i] = adv
	}
}

// DrawTideRibbon draws the Today header's tide ribbon as a miniature of the Focus Clock sea:
// the same three glyphs (~ ≈ ≋), filled continuously in three rows, the same wave and the same
// slow drift between moods. Like the sea it is glyphs only — no tint, no glow, no strokes.
// The water stands full while idle and drains with the round's remaining time.
func (s *SeaState) DrawTideRibbon(c *ui.Context, box ui.Element, v SeaView) {
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		w := float64(r.W)
		h := float64(r.H)
		if w <= 16 || h <= 8 {
			return
		}
		now := v.Now
		if now.IsZero() {
			now = time.Now()
		}

		s.mu.Lock()
		s.ensureRibbonGlyphs()
		glyphs := s.ribbonGlyphs
		ascents := s.ribbonAscents
		advances := s.ribbonAdvances
		pulseAt := s.scene.taskPulseAt
		s.mu.Unlock()

		// Task-completion sweep: a swell that runs left to right once
		pulseActive := false
		pulseU := -1.0
		pulseStrength := 0.0
		if !pulseAt.IsZero() && !v.ReduceMotion && v.Motion != "off" {
			elapsed := now.Sub(pulseAt).Seconds()
			const pulseDur = 1.05
			if elapsed >= 0 && elapsed < pulseDur {
				frac := elapsed / pulseDur
				ease := 1 - (1-frac)*(1-frac)
				pulseActive = true
				pulseU = ease*1.16 - 0.08
				pulseStrength = math.Sin(frac * math.Pi)
			}
		}

		// Motion pacing: slow breathing when idle, steady flow while a round runs, still when paused
		staticMode := v.ReduceMotion || v.Motion == "off"
		animate := !staticMode && (v.Running || pulseActive || v.PointerIn || !v.Paused)
		tt := 1.4
		mood := moodFamiliar
		if animate {
			tSec := float64(p.Now().UnixNano()) / 1e9
			mood = seaMoodFor("clear", tSec)
			speed := 0.24
			if v.Running {
				speed = 0.85
				if v.RemainSec > 0 && v.RemainSec <= 60 {
					speed = 1.35
				}
			} else if v.Paused {
				speed = 0.0
			}
			if v.Motion == "calm" {
				speed *= 0.5
			}
			if speed > 0 {
				tt = math.Mod(tSec, 3600) * speed
			}
		}

		colors := seaLightColors
		if v.Dark {
			colors = seaDarkColors
			if v.HighContrast {
				colors = seaDarkHCColors
			}
		} else if v.HighContrast {
			colors = seaLightHCColors
		}

		// Water level: full while idle, draining with the remaining time of a round.
		wl := 0.0
		if v.Running || v.Paused {
			wl = 1.0 - math.Max(0, math.Min(1, v.RemainRatio))
		}

		const rows = 3
		const cellX = 11.0
		rowH := h / rows
		cols := int(math.Ceil(w/cellX)) + 1
		pxLocal := float64(v.PointerX - r.X)

		for row := 0; row < rows; row++ {
			vv := (float64(row) + 0.5) / rows
			baseY := (float64(row) + 0.5) * rowH
			rowShift := math.Mod(float64(row)*0.37, 1.0) * cellX
			for col := -1; col <= cols; col++ {
				x := float64(col)*cellX + rowShift
				if x < 4 || x > w-4 {
					continue
				}
				u := x / w

				// Soft fade into the card's rounded corners
				edgeFade := 1.0
				if u < 0.05 {
					edgeFade = u / 0.05
				} else if u > 0.95 {
					edgeFade = (1.0 - u) / 0.05
				}

				// The ribbon is far wider than it is tall, so the wave repeats a few times across it.
				waveSin := math.Sin(u*mood.K*2.4 + vv*1.5 - tt*1.3)
				H := (vv-wl)*0.62 + 0.40 + 0.10*(mood.Amp/0.14)*waveSin

				// Pointer lift and completion sweep raise the water locally
				if v.PointerIn {
					if dx := math.Abs(x - pxLocal); dx < 68 {
						f := 1.0 - dx/68.0
						H += 0.16 * f * f
					}
				}
				sweep := 0.0
				if pulseActive {
					du := u - pulseU
					sweep = math.Exp(-(du*du)/(2*0.085*0.085)) * pulseStrength
					H += 0.22 * sweep
				}

				if H < seaSurfaceH {
					continue
				}
				if H > 1 {
					H = 1
				}
				depth := (H - seaSurfaceH) / (1 - seaSurfaceH)
				idx := min(2, int(depth*3))
				// One step lighter than the full sea: this is a quiet strip under a header, not a stage.
				ci := min(len(colors)-2, 2+int(math.Round(depth*float64(len(colors)-4))))
				alpha := (0.46 + 0.40*depth + 0.25*sweep) * edgeFade
				if v.HighContrast {
					alpha *= 1.2
				}
				if alpha > 1 {
					alpha = 1
				}
				if alpha < 0.04 {
					continue
				}
				gx := r.X + float32(x) - advances[idx]/2
				gy := r.Y + float32(baseY+waveSin*1.4-sweep*1.8) + ascents[idx]*0.36
				p.Glyphs(glyphs[idx], gx, gy, colors[ci].Alpha(float32(alpha)))
			}
		}

		if animate {
			if pulseActive || v.PointerIn {
				p.AnimationFrameRate(45)
			} else if v.Running {
				if v.Motion == "calm" {
					p.AnimationFrameRate(24)
				} else {
					p.AnimationFrameRate(36)
				}
			} else {
				p.AnimationFrameRate(20)
			}
		}
	})
}

// DrawCalmWave draws serene static ASCII wave ripples for empty states (zero animation cost).
func (s *SeaState) DrawCalmWave(p *ui.Painter, r ui.Rect, dark bool) {
	w := float64(r.W)
	h := float64(r.H)
	if w <= 20 || h <= 10 {
		return
	}
	cell := 13
	s.mu.Lock()
	s.ensureGlyphs(cell)
	glyphs := s.glyphs
	ascents := s.ascents
	advances := s.advances
	s.mu.Unlock()

	colors := seaLightColors
	if dark {
		colors = seaDarkColors
	}
	cols := max(1, int(math.Ceil(w/float64(cell))))
	rows := min(4, max(1, int(math.Ceil(h/float64(cell)))))
	cellF := float64(cell)
	half := cellF / 2.0
	n := len(seaRunes) - 1

	for row := 0; row < rows; row++ {
		y := float64(row)*cellF + half
		vv := float64(row+1) / float64(rows+1)
		for col := 0; col < cols; col++ {
			x := float64(col)*cellF + half
			uu := x / w
			// Soft bell-curve horizontal fade so ripples taper cleanly on both ends
			edge := math.Sin(uu * math.Pi)
			H := edge * (0.28 + 0.52*vv + 0.18*math.Sin(uu*6.5+float64(row)*1.1))
			if H <= 0.14 {
				continue
			}
			if H > 1 {
				H = 1
			}
			idx := max(1, min(n-1, int(math.Floor(H*float64(n-1)))))
			ci := min(len(colors)-2, max(2, int(math.Round(H*float64(len(colors)-2)))))
			alpha := float32(0.22 + 0.42*H*edge)
			gx := r.X + float32(x) - advances[idx]/2
			gy := r.Y + float32(y-half) + ascents[idx] + float32(cellF-float64(ascents[idx]))*0.18
			p.Glyphs(glyphs[idx], gx, gy, colors[ci].Alpha(alpha))
		}
	}
}
