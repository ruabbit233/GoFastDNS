package benchmark

import (
	"GoFastDNS/internal/config"
	"math"
	"sort"
	"time"
)

func calculateDurationStats(samples []time.Duration) DurationStats {
	values := make([]time.Duration, 0, len(samples))
	for _, sample := range samples {
		if sample > 0 {
			values = append(values, sample)
		}
	}
	if len(values) == 0 {
		return DurationStats{}
	}

	var total time.Duration
	min := values[0]
	max := values[0]
	for _, value := range values {
		total += value
		if value < min {
			min = value
		}
		if value > max {
			max = value
		}
	}

	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	return DurationStats{
		Count:  len(values),
		Min:    min,
		Max:    max,
		Avg:    total / time.Duration(len(values)),
		Median: percentile(sorted, 50),
		P95:    percentile(sorted, 95),
		Jitter: calculateJitter(values),
	}
}

func percentile(sorted []time.Duration, pct int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if pct <= 0 {
		return sorted[0]
	}
	if pct >= 100 {
		return sorted[len(sorted)-1]
	}
	if pct == 50 && len(sorted)%2 == 0 {
		right := len(sorted) / 2
		return (sorted[right-1] + sorted[right]) / 2
	}

	index := int(math.Ceil(float64(pct)/100*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func calculateJitter(samples []time.Duration) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	var total time.Duration
	for _, sample := range samples {
		total += sample
	}
	avg := total / time.Duration(len(samples))

	var totalDeviation time.Duration
	for _, sample := range samples {
		totalDeviation += absDuration(sample - avg)
	}
	return totalDeviation / time.Duration(len(samples))
}

func calculateSequentialJitter(samples []time.Duration) time.Duration {
	if len(samples) < 2 {
		return 0
	}
	var total time.Duration
	for i := 1; i < len(samples); i++ {
		total += absDuration(samples[i] - samples[i-1])
	}
	return total / time.Duration(len(samples)-1)
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func applyResolveScores(results []BenchmarkResult, weights config.ScoreConfig) {
	minDNS, maxDNS := durationRange(results, func(result BenchmarkResult) time.Duration {
		return result.AvgResponseTime
	})
	minPing, maxPing := durationRange(results, func(result BenchmarkResult) time.Duration {
		return result.AvgPingRTT
	})

	for i := range results {
		dnsComponent := durationScore(results[i].AvgResponseTime, minDNS, maxDNS)
		pingComponent := durationScore(results[i].AvgPingRTT, minPing, maxPing)
		successComponent := (results[i].DNSSuccessRate + results[i].PingSuccessRate) / 2
		results[i].Score = weightedScore(weights.DNSWeight, dnsComponent, weights.PingWeight, pingComponent, weights.SuccessWeight, successComponent)
	}
}

func applyDNSPingScores(results []DNSPingBenchmarkResult, weights config.ScoreConfig) {
	minRTT, maxRTT := dnsPingDurationRange(results)
	latencyWeight := weights.DNSWeight + weights.PingWeight
	for i := range results {
		pingComponent := durationScore(results[i].RTT, minRTT, maxRTT)
		results[i].Score = weightedScore(0, 0, latencyWeight, pingComponent, weights.SuccessWeight, results[i].SuccessRate)
	}
}

func durationRange(results []BenchmarkResult, value func(BenchmarkResult) time.Duration) (time.Duration, time.Duration) {
	var min time.Duration
	var max time.Duration
	for _, result := range results {
		current := value(result)
		if current <= 0 {
			continue
		}
		if min == 0 || current < min {
			min = current
		}
		if current > max {
			max = current
		}
	}
	return min, max
}

func dnsPingDurationRange(results []DNSPingBenchmarkResult) (time.Duration, time.Duration) {
	var min time.Duration
	var max time.Duration
	for _, result := range results {
		if result.RTT <= 0 {
			continue
		}
		if min == 0 || result.RTT < min {
			min = result.RTT
		}
		if result.RTT > max {
			max = result.RTT
		}
	}
	return min, max
}

func durationScore(value, min, max time.Duration) float64 {
	if value <= 0 || min <= 0 || max <= 0 {
		return 0
	}
	if min == max {
		return 1
	}
	score := 1 - (float64(value-min) / float64(max-min))
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func weightedScore(weightA, valueA, weightB, valueB, weightC, valueC float64) float64 {
	totalWeight := weightA + weightB + weightC
	if totalWeight <= 0 {
		return 0
	}
	score := (weightA*valueA + weightB*valueB + weightC*valueC) / totalWeight
	return score * 100
}
