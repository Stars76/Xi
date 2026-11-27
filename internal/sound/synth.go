package sound

import (
	"math"
	"sync"

	"github.com/egoist/mygo/audio"
)

const sampleRate = 48000

type note struct {
	freq  float64
	start float64
}

// Player synthesizes chime tones in memory without audio asset files.
type Player struct {
	mu     sync.Mutex
	sounds map[string]*audio.Sound
}

func NewPlayer() *Player {
	return &Player{sounds: map[string]*audio.Sound{}}
}

func (p *Player) Play(kind string) {
	if p == nil {
		return
	}
	s := p.getOrBuild(kind)
	if s != nil {
		s.Play()
	}
}

func (p *Player) getOrBuild(kind string) *audio.Sound {
	p.mu.Lock()
	defer p.mu.Unlock()
	if s, ok := p.sounds[kind]; ok {
		return s
	}
	s := synthKind(kind)
	p.sounds[kind] = s
	return s
}

func synthKind(kind string) *audio.Sound {
	// Delicate water-droplet chirp for clicking fishing boat / breaching whale
	if kind == "droplet" {
		dur := 0.14
		total := int(math.Ceil(dur * sampleRate))
		out := make([]float32, total)
		phase := 0.0
		for i := 0; i < total; i++ {
			t := float64(i) / sampleRate
			frac := t / dur
			freq := 520.0 + 760.0*frac*frac
			phase += 2 * math.Pi * freq / sampleRate
			env := math.Sin(math.Min(1, t/0.012)*math.Pi*0.5) * math.Pow(0.004, frac)
			out[i] = float32(math.Sin(phase) * 0.24 * env)
		}
		return audio.NewSound(sampleRate, 1, out)
	}
	// Ethereal gentle wind-chime harmonic for clicking pirate ship / lighthouse / aurora
	if kind == "chime" {
		dur := 0.42
		total := int(math.Ceil(dur * sampleRate))
		out := make([]float32, total)
		for i := 0; i < total; i++ {
			t := float64(i) / sampleRate
			frac := t / dur
			env := math.Sin(math.Min(1, t/0.014)*math.Pi*0.5) * math.Pow(0.002, frac)
			s := 0.65*math.Sin(2*math.Pi*1174.66*t) + 0.35*math.Sin(2*math.Pi*1760.0*t)
			out[i] = float32(s * 0.20 * env)
		}
		return audio.NewSound(sampleRate, 1, out)
	}

	var seq []note
	dur := 0.60
	gain := 0.22
	switch kind {
	case "break":
		seq = []note{{1046.50, 0}, {783.99, 0.16}, {659.25, 0.32}}
		dur = 0.55
	case "warn":
		seq = []note{{880, 0}, {880, 0.18}}
		gain = 0.16
	default:
		seq = []note{{659.25, 0}, {783.99, 0.14}, {1046.50, 0.28}, {1318.51, 0.42}}
	}

	totalSec := 0.0
	for _, n := range seq {
		if n.start+dur+0.06 > totalSec {
			totalSec = n.start + dur + 0.06
		}
	}
	totalSamples := int(math.Ceil(totalSec * sampleRate))
	mix := make([]float64, totalSamples)
	for _, n := range seq {
		startIdx := int(n.start * sampleRate)
		lenIdx := int(dur * sampleRate)
		attack := int(0.02 * sampleRate)
		for i := 0; i < lenIdx && startIdx+i < totalSamples; i++ {
			t := float64(i) / sampleRate
			env := 1.0
			if i < attack {
				env = float64(i) / float64(max(1, attack))
			} else {
				rem := (t - 0.02) / (dur - 0.02)
				env = math.Pow(0.0005, rem)
			}
			sample := math.Sin(2*math.Pi*n.freq*t) * gain * env
			mix[startIdx+i] += sample
		}
	}

	out := make([]float32, totalSamples)
	for i, v := range mix {
		if v > 0.95 {
			v = 0.95
		} else if v < -0.95 {
			v = -0.95
		}
		out[i] = float32(v)
	}
	return audio.NewSound(sampleRate, 1, out)
}
