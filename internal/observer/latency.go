package observer

import (
	"fmt"
	"hash/fnv"
	"math"
	"time"
)

// Latency is how long the emulated gateway takes to answer a charge: a
// log-normal distribution, like most network services, drawn
// deterministically from the order_ref so a replayed run sees the same delays.
type Latency struct {
	Median time.Duration
	P99    time.Duration
}

// z99 is the standard normal quantile of 0.99.
const z99 = 2.3263478740408408

// For returns the delay for orderRef (zero when Median is zero).
func (l Latency) For(orderRef string) time.Duration {
	if l.Median <= 0 {
		return 0
	}
	sigma := 0.0
	if l.P99 > l.Median {
		sigma = math.Log(float64(l.P99)/float64(l.Median)) / z99
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(orderRef))
	u := (float64(mix64(h.Sum64())>>11) + 0.5) / (1 << 53) // uniform in (0, 1)
	return time.Duration(float64(l.Median) * math.Exp(sigma*normQuantile(u)))
}

// mix64 is the splitmix64 finalizer: FNV alone spreads refs that differ in
// one character ("run-1", "run-2") poorly across the high bits.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// ParseLatency reads "median,p99" durations (e.g. "600ms,2.5s"); "" is none.
func ParseLatency(s string) (Latency, error) {
	if s == "" {
		return Latency{}, nil
	}
	var med, p99 string
	if _, err := fmt.Sscanf(replaceComma(s), "%s %s", &med, &p99); err != nil {
		return Latency{}, fmt.Errorf("observer: latency %q: want median,p99", s)
	}
	m, err1 := time.ParseDuration(med)
	p, err2 := time.ParseDuration(p99)
	if err1 != nil || err2 != nil || m <= 0 || p < m {
		return Latency{}, fmt.Errorf("observer: latency %q: want median,p99 with 0 < median <= p99", s)
	}
	return Latency{Median: m, P99: p}, nil
}

func replaceComma(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c == ',' {
			b[i] = ' '
		}
	}
	return string(b)
}

// normQuantile is the inverse standard normal CDF (Acklam's rational
// approximation, relative error < 1.2e-9).
func normQuantile(p float64) float64 {
	a := [6]float64{-3.969683028665376e+01, 2.209460984245205e+02, -2.759285104469687e+02, 1.383577518672690e+02, -3.066479806614716e+01, 2.506628277459239e+00}
	b := [5]float64{-5.447609879822406e+01, 1.615858368580409e+02, -1.556989798598866e+02, 6.680131188771972e+01, -1.328068155288572e+01}
	c := [6]float64{-7.784894002430293e-03, -3.223964580411365e-01, -2.400758277161838e+00, -2.549732539343734e+00, 4.374664141464968e+00, 2.938163982698783e+00}
	d := [4]float64{7.784695709041462e-03, 3.224671290700398e-01, 2.445134137142996e+00, 3.754408661907416e+00}
	const lo, hi = 0.02425, 1 - 0.02425
	switch {
	case p < lo:
		q := math.Sqrt(-2 * math.Log(p))
		return (((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	case p > hi:
		q := math.Sqrt(-2 * math.Log(1-p))
		return -(((((c[0]*q+c[1])*q+c[2])*q+c[3])*q+c[4])*q + c[5]) / ((((d[0]*q+d[1])*q+d[2])*q+d[3])*q + 1)
	default:
		q := p - 0.5
		r := q * q
		return (((((a[0]*r+a[1])*r+a[2])*r+a[3])*r+a[4])*r + a[5]) * q / (((((b[0]*r+b[1])*r+b[2])*r+b[3])*r+b[4])*r + 1)
	}
}
