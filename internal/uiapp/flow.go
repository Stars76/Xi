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
// The sea drifts slowly from one mood to the next so it is never the same wave for long.
type seaMood struct{ K, Amp, Speed float64 }

var seaMoods = []seaMood{
	{K: 4.2, Amp: 0.14, Speed: 1.0},  // the familiar swell
	{K: 3.2, Amp: 0.09, Speed: 0.6},  // calm
	{K: 2.6, Amp: 0.17, Speed: 0.85}, // long rollers
	{K: 5.6, Amp: 0.11, Speed: 1.15}, // fine, quicker waves
}

const (
	seaMoodHold   = 90.0  // seconds per mood (roughly half held, half gliding to the next)
	seaSwellEvery = 240.0 // a broad swell crosses the surface about this often
	seaSwellPass  = 26.0  // and takes this long to cross
)

func smoothstep01(x float64) float64 {
	x = math.Max(0, math.Min(1, x))
	return x * x * (3 - 2*x)
}

// seaMoodAt blends the two moods around clock (seconds of animated time).
func seaMoodAt(clock float64) seaMood {
	ph := clock / seaMoodHold
	i := int(math.Floor(ph))
	a, b := seaMoods[i%len(seaMoods)], seaMoods[(i+1)%len(seaMoods)]
	f := smoothstep01((ph - math.Floor(ph) - 0.45) / 0.55)
	return seaMood{a.K + (b.K-a.K)*f, a.Amp + (b.Amp-a.Amp)*f, a.Speed + (b.Speed-a.Speed)*f}
}

// seaSwellAt returns the centre (0..1 across the width, may be off-screen) and strength (0..1)
// of the occasional broad swell at clock.
func seaSwellAt(clock float64) (centre, strength float64) {
	tIn := math.Mod(clock, seaSwellEvery) - (seaSwellEvery - seaSwellPass - 10)
	if tIn < 0 || tIn > seaSwellPass {
		return 0, 0
	}
	f := tIn / seaSwellPass
	return -0.3 + f*1.6, smoothstep01(f/0.25) * smoothstep01((1-f)/0.25)
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
			if v.Weather == "storm" {
				speed *= 1.25
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
					s.wavePhase += dt * speed * seaMoodAt(s.moodClock).Speed
					s.moodClock += dt
				}
			}
			s.lastPaintAt = paintNow
		} else {
			s.lastPaintAt = time.Time{}
		}
		tt := s.wavePhase
		mood := seaMoodAt(s.moodClock)
		swellU, swellAmt := seaSwellAt(s.moodClock)
		levelShown := s.levelShown

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
				H := rowBase + mood.Amp*waveSin
				if swellAmt > 0 {
					du := (uu - swellU) / 0.24
					H += 0.086 * swellAmt * math.Exp(-du*du) // the passing swell lifts the surface locally
				}
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

// Minimal whisper-soft horizontal wave glyphs for the Today 526Hz tide ribbon (zero noisy/vertical symbols).
var ribbonRunes = []string{"·", "~", "≈"}

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

// DrawTideRibbon renders a restrained, airy 526Hz acoustic tide ribbon along the bottom edge
// of the Today header card: pure paper above, whisper-soft · ~ ∿ harmonic ripples below.
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

		// Soft vertical paper-to-shallow-water wash (melts seamlessly from the paper above)
		tintBottom := ui.Hex("#EAF4FF").Alpha(0.76)
		if v.Dark {
			tintBottom = ui.Hex("#17283D").Alpha(0.86)
		}
		p.FillGradient(r, ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{
				{Color: ui.Transparent, At: 0.0},
				{Color: tintBottom.Alpha(0.32), At: 0.42},
				{Color: tintBottom, At: 1.0},
			},
		}, 0)

		s.mu.Lock()
		s.ensureRibbonGlyphs()
		glyphs := s.ribbonGlyphs
		ascents := s.ribbonAscents
		advances := s.ribbonAdvances
		pulseAt := s.scene.taskPulseAt
		s.mu.Unlock()

		// Check active task-completion left-to-right luminous wave sweep
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

		// Motion pacing: calm quiet breathing when idle, smooth acoustic flow when running
		staticMode := v.ReduceMotion || v.Motion == "off"
		animate := !staticMode && (v.Running || pulseActive || v.PointerIn || !v.Paused)
		tt := 1.4
		if animate {
			tSec := float64(p.Now().UnixNano()) / 1e9
			speed := 0.24 // whisper-slow idle breathing
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
				tt = tSec * speed
			}
		}

		// Optional soft luminous wave band during task-completion sweep
		if pulseActive {
			bandW := float32(math.Max(64, w*0.18))
			cx := r.X + float32(pulseU*w)
			glowCol := ui.Hex("#007CFF")
			if v.Dark {
				glowCol = ui.Hex("#66B5FF")
			}
			p.FillGradient(ui.Rect{X: cx - bandW/2, Y: r.Y + 2, W: bandW, H: r.H - 2}, ui.LinearGradient{
				Angle: 90,
				Stops: []ui.GradientStop{
					{Color: ui.Transparent, At: 0.0},
					{Color: glowCol.Alpha(float32(0.18 * pulseStrength)), At: 0.5},
					{Color: ui.Transparent, At: 1.0},
				},
			}, 0)
		}

		// Restrained ocean-blue tones (zero dark ink / zero visual harshness)
		var colMist, colMid, colCrest ui.Color
		if v.Dark {
			colMist = ui.Hex("#376FA8")
			colMid = ui.Hex("#56A5F5")
			colCrest = ui.Hex("#88C8FF")
		} else {
			colMist = ui.Hex("#89C6FF")
			colMid = ui.Hex("#3696FF")
			colCrest = ui.Hex("#007CFF")
		}

		elapsedRatio := 1.0 - math.Max(0, math.Min(1, v.RemainRatio))
		pxLocal := float64(v.PointerX - r.X)

		// 3 staggered horizontal strata inside the 32px ribbon (varied spacing eliminates vertical grid lines):
		// Row 0 (mist crest), Row 1 (primary 526Hz acoustic wave), Row 2 (soft undertow)
		type stratum struct {
			yFrac     float64
			cellX     float64
			xShift    float64
			freqA     float64
			freqB     float64
			phaseMul  float64
			phaseOff  float64
			yAmp      float64
			baseAlpha float64
			minThresh float64
		}
		strata := []stratum{
			{yFrac: 0.25, cellX: 19.0, xShift: 7.0, freqA: 2.6, freqB: 5.4, phaseMul: 0.72, phaseOff: 0.8, yAmp: 1.5, baseAlpha: 0.17, minThresh: 0.44},
			{yFrac: 0.56, cellX: 16.0, xShift: 0.0, freqA: 3.1, freqB: 6.4, phaseMul: 1.00, phaseOff: 0.0, yAmp: 2.3, baseAlpha: 0.31, minThresh: 0.08},
			{yFrac: 0.85, cellX: 18.0, xShift: 11.0, freqA: 2.4, freqB: 4.9, phaseMul: -0.62, phaseOff: 2.1, yAmp: 1.3, baseAlpha: 0.22, minThresh: 0.20},
		}

		for si, st := range strata {
			baseY := h * st.yFrac
			cols := max(1, int(math.Floor((w-16)/st.cellX)))
			startX := (w-float64(cols-1)*st.cellX)*0.5 + st.xShift*0.5
			for col := 0; col < cols; col++ {
				x := startX + float64(col)*st.cellX
				if x < 8 || x > w-8 {
					continue
				}
				u := x / w

				// Silky horizontal fade at both left & right edges
				edgeFade := 1.0
				if u < 0.07 {
					edgeFade = u / 0.07
				} else if u > 0.93 {
					edgeFade = (1.0 - u) / 0.07
				}
				edgeFade = edgeFade * edgeFade * (3 - 2*edgeFade)

				// Coherent dual-harmonic 526Hz acoustic sine wave
				wA := math.Sin(u*st.freqA*math.Pi - tt*st.phaseMul + st.phaseOff)
				wB := math.Sin(u*st.freqB*math.Pi + tt*st.phaseMul*0.58 + st.phaseOff*0.5)
				amp := 0.5 + 0.32*wA + 0.18*wB // [0, 1]

				// Subtle pointer lift
				hoverBoost := 0.0
				if v.PointerIn {
					dx := math.Abs(x - pxLocal)
					if dx < 68 {
						f := 1.0 - dx/68.0
						hoverBoost = 0.28 * f * f
					}
				}

				// Task-completion luminous wave sweep boost
				sweepBoost := 0.0
				if pulseActive {
					du := u - pulseU
					g := math.Exp(-(du * du) / (2 * 0.085 * 0.085))
					sweepBoost = g * pulseStrength * 0.65
				}

				energy := math.Min(1.0, amp+hoverBoost+sweepBoost)
				if energy < st.minThresh {
					continue
				}

				// Choose airy glyph by wave energy: · (trough) -> ~ (swell) -> ∿ (crest)
				idx := 0
				if energy >= 0.72 || (si == 1 && energy >= 0.64) {
					idx = 2 // ∿
				} else if energy >= 0.42 {
					idx = 1 // ~
				}

				// Color & alpha: whisper-calm in idle, gently enriched along focus elapsed progress
				colTone := colMist
				if idx == 1 {
					colTone = colMid
				} else if idx == 2 {
					colTone = colCrest
				}

				alpha := st.baseAlpha * (0.55 + 0.85*energy)
				if v.Running {
					if u <= elapsedRatio+0.02 {
						alpha *= 1.38
						if idx == 0 {
							colTone = colMid
						}
					} else {
						alpha *= 0.88
					}
				}
				if v.Dark {
					alpha *= 1.18
				}
				if v.HighContrast {
					alpha *= 1.25
				}
				alpha += sweepBoost * 0.42
				alpha *= edgeFade
				if alpha > 0.72 {
					alpha = 0.72
				}
				if alpha < 0.04 {
					continue
				}

				// Smooth harmonic vertical undulation (eliminates boxy terminal-grid stiffness)
				yOff := (amp - 0.5) * 2.0 * st.yAmp
				if sweepBoost > 0.02 {
					yOff -= sweepBoost * 2.2
				}
				gx := r.X + float32(x) - advances[idx]/2
				gy := r.Y + float32(baseY+yOff) + ascents[idx]*0.36
				p.Glyphs(glyphs[idx], gx, gy, colTone.Alpha(float32(alpha)))
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
