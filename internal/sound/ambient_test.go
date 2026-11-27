package sound

import (
	"math"
	"testing"
)

func TestAmbientDecoderAllWeatherModes(t *testing.T) {
	for _, mode := range []string{"clear", "rain", "storm", "snow", "fog"} {
		dec := newAmbientDecoder()
		dec.setState(mode, true)
		buf := make([]float32, 4096)
		// Warm up past ~220ms fade-in (~10,560 frames -> 6 buffers of 2048 stereo frames)
		for k := 0; k < 8; k++ {
			n, err := dec.Read(buf)
			if err != nil || n != len(buf) {
				t.Fatalf("mode %s read failed: n=%d err=%v", mode, n, err)
			}
		}
		// Measure RMS across 2 full seconds so slow wave swells are fairly averaged
		var sumSq float64
		var peak float64
		totalSamples := 0
		for k := 0; k < 48; k++ {
			n, err := dec.Read(buf)
			if err != nil || n != len(buf) {
				t.Fatalf("mode %s read failed: n=%d err=%v", mode, n, err)
			}
			for _, s := range buf {
				v := float64(s)
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("mode %s emitted invalid sample %v", mode, s)
				}
				if math.Abs(v) > 1.0 {
					t.Fatalf("mode %s clipped sample %v", mode, s)
				}
				if math.Abs(v) > peak {
					peak = math.Abs(v)
				}
				sumSq += v * v
				totalSamples++
			}
		}
		rms := math.Sqrt(sumSq / float64(totalSamples))
		// Calibrated clearly audible range: ~ -22 dBFS (0.08) to -9 dBFS (0.35) at unity gain
		if rms < 0.08 || rms > 0.38 {
			t.Fatalf("mode %s RMS=%.4f out of comfortable audible range [0.08, 0.38] (peak=%.4f)", mode, rms, peak)
		}
	}
}

func TestSynthMicroFeedbackSounds(t *testing.T) {
	for _, kind := range []string{"focus", "break", "warn", "droplet", "chime"} {
		if s := synthKind(kind); s == nil {
			t.Fatalf("synthKind(%q) returned nil", kind)
		}
	}
}
