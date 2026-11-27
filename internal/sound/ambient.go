package sound

import (
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/audio"
)

const ambientSampleRate = 48000

// ambientDecoder synthesizes infinite, smooth, stereo procedural nature audio
// calibrated to comfortable audible background levels (~ -16 dBFS RMS at unity gain)
// synchronized to the sea's weather states:
// clear = rolling ocean surf, rain = rainfall + droplets, storm = downpour + distant thunder, snow/fog = coastal breeze.
type ambientDecoder struct {
	mu             sync.Mutex
	weather        string
	active         bool
	currentFade    float64
	rngL           uint32
	rngR           uint32
	frame          int64
	lpL1, lpL2     float64
	lpR1, lpR2     float64
	hpL, hpR       float64
	prevWL, prevWR float64
	dropEnvL       float64
	dropEnvR       float64
	dropPhaseL     float64
	dropPhaseR     float64
	dropFreqL      float64
	dropFreqR      float64
	thunderEnv     float64
	thunderNext    int64
	thunderLP1     float64
	thunderLP2     float64
}

func newAmbientDecoder() *ambientDecoder {
	return &ambientDecoder{
		weather:     "clear",
		rngL:        0x6d2b79f5,
		rngR:        0xa5a3564e,
		thunderNext: ambientSampleRate * 4,
		dropFreqL:   1200,
		dropFreqR:   1500,
	}
}

func (d *ambientDecoder) SampleRate() int { return ambientSampleRate }
func (d *ambientDecoder) Channels() int   { return 2 }
func (d *ambientDecoder) Len() int64      { return -1 }

func (d *ambientDecoder) SetPosition(frame int64) error {
	d.mu.Lock()
	// Resetting position flushes mygo/audio's 1s ring buffer for instant weather/play switches;
	// keep gentle fade continuity without resetting RNG phase awkwardly.
	if frame == 0 && d.active && d.currentFade < 0.05 {
		d.currentFade = 0
	}
	d.mu.Unlock()
	return nil
}

func (d *ambientDecoder) setState(weather string, active bool) (weatherChanged bool) {
	if weather == "" {
		weather = "clear"
	}
	d.mu.Lock()
	weatherChanged = d.weather != weather
	d.weather = weather
	d.active = active
	d.mu.Unlock()
	return weatherChanged
}

func nextWhite(state *uint32) float64 {
	x := *state
	x ^= x << 13
	x ^= x >> 17
	x ^= x << 5
	*state = x
	// Uniform in [-1, 1]
	return (float64(x)/2147483648.0 - 1.0)
}

func (d *ambientDecoder) Read(buf []float32) (int, error) {
	frames := len(buf) / 2
	if frames <= 0 {
		return 0, nil
	}

	d.mu.Lock()
	weather := d.weather
	active := d.active
	fade := d.currentFade
	rngL, rngR := d.rngL, d.rngR
	lpL1, lpL2 := d.lpL1, d.lpL2
	lpR1, lpR2 := d.lpR1, d.lpR2
	hpL, hpR := d.hpL, d.hpR
	prevWL, prevWR := d.prevWL, d.prevWR
	dropEnvL, dropEnvR := d.dropEnvL, d.dropEnvR
	dropPhaseL, dropPhaseR := d.dropPhaseL, d.dropPhaseR
	dropFreqL, dropFreqR := d.dropFreqL, d.dropFreqR
	thEnv, thNext, thLP1, thLP2 := d.thunderEnv, d.thunderNext, d.thunderLP1, d.thunderLP2
	frame := d.frame
	d.mu.Unlock()

	target := 0.0
	if active {
		target = 1.0
	}
	// Snappy ~220ms silky fade-in / ~350ms smooth fade-out so preview and focus start respond immediately.
	const fadeInStep = 1.0 / (0.22 * ambientSampleRate)
	const fadeOutStep = 1.0 / (0.35 * ambientSampleRate)

	for i := 0; i < frames; i++ {
		if fade < target {
			fade += fadeInStep
			if fade > target {
				fade = target
			}
		} else if fade > target {
			fade -= fadeOutStep
			if fade < target {
				fade = target
			}
		}

		if fade < 1e-5 && target < 1e-5 {
			buf[i*2] = 0
			buf[i*2+1] = 0
			frame++
			continue
		}

		wL := nextWhite(&rngL)
		wR := nextWhite(&rngR)
		t := float64(frame) / ambientSampleRate

		var outL, outR float64
		switch weather {
		case "rain":
			// Soothing rainfall bed (balanced pink-ish midrange + soft low body) + resonant stereo water droplets
			lpL1 += 0.24 * (wL - lpL1)
			lpR1 += 0.24 * (wR - lpR1)
			hpL = 0.84 * (hpL + lpL1 - prevWL)
			hpR = 0.84 * (hpR + lpR1 - prevWR)
			prevWL, prevWR = lpL1, lpR1
			lpL2 += 0.032 * (wL - lpL2)
			lpR2 += 0.032 * (wR - lpR2)

			// Occasional delicate water-drop plinks across stereo field
			if dropEnvL < 0.002 && (rngL&0x1FFF) == 7 {
				dropEnvL = 0.22
				dropPhaseL = 0
				dropFreqL = 900 + float64(rngL&0x3FF)
			}
			if dropEnvR < 0.002 && (rngR&0x1FFF) == 13 {
				dropEnvR = 0.22
				dropPhaseR = 0
				dropFreqR = 950 + float64(rngR&0x3FF)
			}
			dropSigL, dropSigR := 0.0, 0.0
			if dropEnvL > 0.001 {
				dropPhaseL += 2 * math.Pi * dropFreqL / ambientSampleRate
				dropSigL = math.Sin(dropPhaseL) * dropEnvL
				dropEnvL *= 0.9965
			}
			if dropEnvR > 0.001 {
				dropPhaseR += 2 * math.Pi * dropFreqR / ambientSampleRate
				dropSigR = math.Sin(dropPhaseR) * dropEnvR
				dropEnvR *= 0.9965
			}
			outL = hpL*0.78 + lpL2*1.75 + dropSigL
			outR = hpR*0.78 + lpR2*1.75 + dropSigR

		case "storm":
			// Immersive heavy rain + deep rolling distant thunder rumbles
			lpL1 += 0.28 * (wL - lpL1)
			lpR1 += 0.28 * (wR - lpR1)
			hpL = 0.82 * (hpL + lpL1 - prevWL)
			hpR = 0.82 * (hpR + lpR1 - prevWR)
			prevWL, prevWR = lpL1, lpR1
			lpL2 += 0.028 * (wL - lpL2)
			lpR2 += 0.028 * (wR - lpR2)

			if frame >= thNext {
				thEnv = 1.0
				thNext = frame + int64(ambientSampleRate*(8+int(rngL%9)))
			}
			rumble := 0.0
			if thEnv > 0.002 {
				thLP1 += 0.018 * (wL - thLP1)
				thLP2 += 0.014 * (thLP1 - thLP2)
				rumble = thLP2 * thEnv * 8.5
				thEnv *= 0.999962
			}
			outL = hpL*0.85 + lpL2*2.05 + rumble
			outR = hpR*0.85 + lpR2*2.05 + rumble*0.92

		case "snow", "fog":
			// Airy, silky coastal breeze with slow stereo breathing modulation (clearly audible, never harsh)
			lpL1 += 0.060 * (wL - lpL1)
			lpL2 += 0.042 * (lpL1 - lpL2)
			lpR1 += 0.060 * (wR - lpR1)
			lpR2 += 0.042 * (lpR1 - lpR2)
			breathL := 0.52 + 0.48*math.Sin(2*math.Pi*0.11*t)
			breathR := 0.52 + 0.48*math.Sin(2*math.Pi*0.11*t+0.85)
			airyL := (lpL1 - lpL2) * 1.45 * breathL
			airyR := (lpR1 - lpR2) * 1.45 * breathR
			outL = lpL2*3.4*breathL + airyL
			outR = lpR2*3.4*breathR + airyR

		default: // clear -> warm, lush rolling stereo ocean surf waves
			lpL1 += 0.085 * (wL - lpL1)
			lpL2 += 0.040 * (lpL1 - lpL2)
			lpR1 += 0.085 * (wR - lpR1)
			lpR2 += 0.040 * (lpR1 - lpR2)
			// Organic ocean surf swell (period ~7.6s) washing smoothly across stereo L/R
			swellL := 0.36 + 0.64*math.Pow(0.5+0.5*math.Sin(2*math.Pi*0.131*t), 1.5)
			swellR := 0.36 + 0.64*math.Pow(0.5+0.5*math.Sin(2*math.Pi*0.131*t+0.48), 1.5)
			// Crisp airy shore foam hiss peaking on each wave crest
			foamL := (lpL1 - lpL2) * swellL * 2.1
			foamR := (lpR1 - lpR2) * swellR * 2.1
			// Deep rolling water body + airy shore crest
			outL = lpL2*3.6*swellL + foamL
			outR = lpR2*3.6*swellR + foamR
		}

		// Warm soft limiter so peaks stay smooth and never hard-clip
		sL := math.Tanh(outL*0.85) * 0.62 * fade
		sR := math.Tanh(outR*0.85) * 0.62 * fade
		buf[i*2] = float32(sL)
		buf[i*2+1] = float32(sR)
		frame++
	}

	d.mu.Lock()
	d.currentFade = fade
	d.rngL, d.rngR = rngL, rngR
	d.lpL1, d.lpL2 = lpL1, lpL2
	d.lpR1, d.lpR2 = lpR1, lpR2
	d.hpL, d.hpR = hpL, hpR
	d.prevWL, d.prevWR = prevWL, prevWR
	d.dropEnvL, d.dropEnvR = dropEnvL, dropEnvR
	d.dropPhaseL, d.dropPhaseR = dropPhaseL, dropPhaseR
	d.dropFreqL, d.dropFreqR = dropFreqL, dropFreqR
	d.thunderEnv, d.thunderNext, d.thunderLP1, d.thunderLP2 = thEnv, thNext, thLP1, thLP2
	d.frame = frame
	d.mu.Unlock()

	return frames * 2, nil
}

// AmbientEngine manages weather-synchronized procedural focus white noise with instant volume & weather switching.
type AmbientEngine struct {
	mu           sync.Mutex
	dec          *ambientDecoder
	player       *audio.Player
	previewUntil time.Time
	previewTimer *time.Timer
	pauseTimer   *time.Timer
	lastWant     bool
	lastWeather  string
	lastVol      float64
	closed       bool
}

func NewAmbientEngine() *AmbientEngine {
	return &AmbientEngine{
		dec:     newAmbientDecoder(),
		lastVol: 0.45,
	}
}

// Sync updates desired playback state (active during running focus or short manual preview).
func (ae *AmbientEngine) Sync(wantPlay bool, weather string, volume float64) {
	if ae == nil {
		return
	}
	if volume < 0 {
		volume = 0
	} else if volume > 1 {
		volume = 1
	}

	ae.mu.Lock()
	defer ae.mu.Unlock()
	if ae.closed {
		return
	}
	ae.lastWant = wantPlay
	ae.lastWeather = weather
	ae.lastVol = volume

	effectivePlay := wantPlay || time.Now().Before(ae.previewUntil)
	weatherChanged := ae.dec.setState(weather, effectivePlay)

	if effectivePlay {
		if ae.pauseTimer != nil {
			ae.pauseTimer.Stop()
			ae.pauseTimer = nil
		}
		if ae.player == nil {
			ae.player = audio.NewPlayer(ae.dec)
			ae.player.SetVolume(volume)
			ae.player.Play()
		} else {
			// SetVolume updates WASAPI mixer in ~10ms with zero ring-buffer lag!
			ae.player.SetVolume(volume)
			wasPlaying := ae.player.Playing()
			if !wasPlaying || weatherChanged {
				// Flush any stale 1s pre-buffered silence or previous weather from mygo/audio's ring buffer!
				ae.player.Seek(0)
			}
			if !wasPlaying {
				ae.player.Play()
			}
		}
	} else if ae.player != nil {
		// Let the decoder complete its smooth ~350ms fade-out, then pause the player so zero CPU is used
		// and no 1-second silence backlog accumulates for the next start!
		if ae.pauseTimer != nil {
			ae.pauseTimer.Stop()
		}
		p := ae.player
		ae.pauseTimer = time.AfterFunc(420*time.Millisecond, func() {
			ae.mu.Lock()
			stillOff := !ae.lastWant && !time.Now().Before(ae.previewUntil)
			ae.pauseTimer = nil
			ae.mu.Unlock()
			if stillOff && p != nil {
				p.Pause()
			}
		})
	}
}

// Preview plays the procedural ambient noise for dur (e.g. 5 seconds) regardless of timer state.
func (ae *AmbientEngine) Preview(weather string, volume float64, dur time.Duration) {
	if ae == nil {
		return
	}
	if dur <= 0 {
		dur = 5 * time.Second
	}
	ae.mu.Lock()
	if ae.closed {
		ae.mu.Unlock()
		return
	}
	ae.previewUntil = time.Now().Add(dur)
	if ae.previewTimer != nil {
		ae.previewTimer.Stop()
	}
	want := ae.lastWant
	ae.previewTimer = time.AfterFunc(dur+80*time.Millisecond, func() {
		ae.mu.Lock()
		w, wt, v := ae.lastWant, ae.lastWeather, ae.lastVol
		ae.mu.Unlock()
		ae.Sync(w, wt, v)
	})
	ae.mu.Unlock()
	ae.Sync(want, weather, volume)
}

// StopPreview immediately ends an active manual preview burst.
func (ae *AmbientEngine) StopPreview() {
	if ae == nil {
		return
	}
	ae.mu.Lock()
	ae.previewUntil = time.Time{}
	if ae.previewTimer != nil {
		ae.previewTimer.Stop()
		ae.previewTimer = nil
	}
	w, wt, v := ae.lastWant, ae.lastWeather, ae.lastVol
	ae.mu.Unlock()
	ae.Sync(w, wt, v)
}

// IsPreviewing reports whether a manual preview burst is currently active.
func (ae *AmbientEngine) IsPreviewing() bool {
	if ae == nil {
		return false
	}
	ae.mu.Lock()
	defer ae.mu.Unlock()
	return time.Now().Before(ae.previewUntil)
}

// Close releases the underlying audio stream cleanly.
func (ae *AmbientEngine) Close() {
	if ae == nil {
		return
	}
	ae.mu.Lock()
	ae.closed = true
	if ae.previewTimer != nil {
		ae.previewTimer.Stop()
		ae.previewTimer = nil
	}
	if ae.pauseTimer != nil {
		ae.pauseTimer.Stop()
		ae.pauseTimer = nil
	}
	p := ae.player
	ae.player = nil
	ae.dec.setState("clear", false)
	ae.mu.Unlock()
	if p != nil {
		p.Close()
	}
}
