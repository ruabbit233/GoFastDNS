package benchmark

import (
	"GoFastDNS/internal/dns"
	"GoFastDNS/internal/ping"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type resolvePingJSONReport struct {
	Mode        string                      `json:"mode"`
	GeneratedAt time.Time                   `json:"generated_at"`
	Summary     []resolvePingJSONSummaryRow `json:"summary"`
	Results     []resolvePingJSONResult     `json:"results"`
}

type resolvePingJSONSummaryRow struct {
	Server              string  `json:"server"`
	AvgResponseTimeMS   float64 `json:"avg_response_time_ms"`
	AvgPingRTTMS        float64 `json:"avg_ping_rtt_ms"`
	SuccessRate         float64 `json:"success_rate"`
	TotalRetries        int     `json:"total_retries"`
	DomainCount         int     `json:"domain_count"`
	SuccessfulDNSCount  int     `json:"successful_dns_count"`
	SuccessfulPingCount int     `json:"successful_ping_count"`
}

type resolvePingJSONResult struct {
	Server            string             `json:"server"`
	AvgResponseTimeMS float64            `json:"avg_response_time_ms"`
	AvgPingRTTMS      float64            `json:"avg_ping_rtt_ms"`
	SuccessRate       float64            `json:"success_rate"`
	TotalRetries      int                `json:"total_retries"`
	Domains           []domainJSONResult `json:"domains"`
}

type domainJSONResult struct {
	Domain         string             `json:"domain"`
	ResponseTimeMS float64            `json:"response_time_ms"`
	RetryCount     int                `json:"retry_count"`
	Error          string             `json:"error,omitempty"`
	QueryErrors    []string           `json:"query_errors,omitempty"`
	ResponseCodes  []dns.ResponseCode `json:"response_codes,omitempty"`
	NoAnswer       bool               `json:"no_answer"`
	Answers        []dns.Answer       `json:"answers"`
	PingTargets    []string           `json:"ping_targets"`
	PingAvgRTTMS   float64            `json:"ping_avg_rtt_ms"`
	PingError      string             `json:"ping_error,omitempty"`
	PingResults    []pingJSONResult   `json:"ping_results"`
}

type pingJSONResult struct {
	IP          string  `json:"ip"`
	RTTMS       float64 `json:"rtt_ms"`
	PacketLoss  float64 `json:"packet_loss"`
	PacketsSent int     `json:"packets_sent"`
	Error       string  `json:"error,omitempty"`
}

type dnsPingJSONReport struct {
	Mode        string              `json:"mode"`
	GeneratedAt time.Time           `json:"generated_at"`
	Summary     dnsPingJSONSummary  `json:"summary"`
	Results     []dnsPingJSONResult `json:"results"`
}

type dnsPingJSONSummary struct {
	TargetCount      int     `json:"target_count"`
	SuccessfulCount  int     `json:"successful_count"`
	AvgRTTMS         float64 `json:"avg_rtt_ms"`
	AvgPacketLossPct float64 `json:"avg_packet_loss_percent"`
}

type dnsPingJSONResult struct {
	Server      string  `json:"server"`
	Target      string  `json:"target"`
	RTTMS       float64 `json:"rtt_ms"`
	PacketLoss  float64 `json:"packet_loss"`
	PacketsSent int     `json:"packets_sent"`
	Error       string  `json:"error,omitempty"`
}

func SaveResolvePingResultsToJSON(results []BenchmarkResult, outputPath string) (string, error) {
	report := resolvePingJSONReport{
		Mode:        "resolve-ping",
		GeneratedAt: time.Now(),
		Summary:     buildResolvePingJSONSummary(results),
		Results:     buildResolvePingJSONResults(results),
	}
	filename := outputFilename(outputPath, "resolve_ping_benchmark", "json")
	if err := writeJSONFile(filename, report); err != nil {
		return "", err
	}
	return filename, nil
}

func SaveDNSPingResultsToJSON(results []DNSPingBenchmarkResult, outputPath string) (string, error) {
	report := dnsPingJSONReport{
		Mode:        "dns-ping",
		GeneratedAt: time.Now(),
		Summary:     buildDNSPingJSONSummary(results),
		Results:     buildDNSPingJSONResults(results),
	}
	filename := outputFilename(outputPath, "dns_ping_benchmark", "json")
	if err := writeJSONFile(filename, report); err != nil {
		return "", err
	}
	return filename, nil
}

func writeJSONFile(filename string, value any) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create json file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("write json file: %w", err)
	}
	return nil
}

func buildResolvePingJSONSummary(results []BenchmarkResult) []resolvePingJSONSummaryRow {
	rows := make([]resolvePingJSONSummaryRow, 0, len(results))
	for _, result := range results {
		row := resolvePingJSONSummaryRow{
			Server:            result.Server,
			AvgResponseTimeMS: durationMS(result.AvgResponseTime),
			AvgPingRTTMS:      durationMS(result.AvgPingRTT),
			SuccessRate:       result.SuccessRate,
			TotalRetries:      result.TotalRetries,
			DomainCount:       len(result.DomainResults),
		}
		for _, domain := range result.DomainResults {
			if domain.Error == nil {
				row.SuccessfulDNSCount++
			}
			if domain.DnsPingResults.AvgRTT > 0 {
				row.SuccessfulPingCount++
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func buildResolvePingJSONResults(results []BenchmarkResult) []resolvePingJSONResult {
	rows := make([]resolvePingJSONResult, 0, len(results))
	for _, result := range results {
		row := resolvePingJSONResult{
			Server:            result.Server,
			AvgResponseTimeMS: durationMS(result.AvgResponseTime),
			AvgPingRTTMS:      durationMS(result.AvgPingRTT),
			SuccessRate:       result.SuccessRate,
			TotalRetries:      result.TotalRetries,
			Domains:           buildDomainJSONResults(result.DomainResults),
		}
		rows = append(rows, row)
	}
	return rows
}

func buildDomainJSONResults(results []DomainResult) []domainJSONResult {
	rows := make([]domainJSONResult, 0, len(results))
	for _, result := range results {
		row := domainJSONResult{
			Domain:         result.Domain,
			ResponseTimeMS: durationMS(result.ResponseTime),
			RetryCount:     result.RetryCount,
			QueryErrors:    result.QueryErrors,
			ResponseCodes:  result.ResponseCodes,
			NoAnswer:       result.NoAnswer,
			Answers:        emptyAnswersAsSlice(result.Answers),
			PingTargets:    pingTargets(result.DnsPingResults.PingResults),
			PingAvgRTTMS:   durationMS(result.DnsPingResults.AvgRTT),
			PingResults:    buildPingJSONResults(result.DnsPingResults.PingResults),
		}
		if result.Error != nil {
			row.Error = result.Error.Error()
		}
		if result.DnsPingResults.Error != nil {
			row.PingError = result.DnsPingResults.Error.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func emptyAnswersAsSlice(answers []dns.Answer) []dns.Answer {
	if answers == nil {
		return []dns.Answer{}
	}
	return answers
}

func buildPingJSONResults(results []ping.PingResult) []pingJSONResult {
	if results == nil {
		return []pingJSONResult{}
	}
	rows := make([]pingJSONResult, 0, len(results))
	for _, result := range results {
		row := pingJSONResult{
			IP:          result.IP,
			RTTMS:       durationMS(result.RTT),
			PacketLoss:  result.PacketLoss,
			PacketsSent: result.PacketsSent,
		}
		if result.Error != nil {
			row.Error = result.Error.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func buildDNSPingJSONSummary(results []DNSPingBenchmarkResult) dnsPingJSONSummary {
	var totalRTT time.Duration
	var totalLoss float64
	var successful int
	for _, result := range results {
		if result.Error != nil {
			continue
		}
		successful++
		totalRTT += result.RTT
		totalLoss += result.PacketLoss
	}

	summary := dnsPingJSONSummary{
		TargetCount:     len(results),
		SuccessfulCount: successful,
	}
	if successful > 0 {
		summary.AvgRTTMS = durationMS(totalRTT / time.Duration(successful))
		summary.AvgPacketLossPct = totalLoss / float64(successful)
	}
	return summary
}

func buildDNSPingJSONResults(results []DNSPingBenchmarkResult) []dnsPingJSONResult {
	rows := make([]dnsPingJSONResult, 0, len(results))
	for _, result := range results {
		row := dnsPingJSONResult{
			Server:      result.Server,
			Target:      result.Target,
			RTTMS:       durationMS(result.RTT),
			PacketLoss:  result.PacketLoss,
			PacketsSent: result.PacketsSent,
		}
		if result.Error != nil {
			row.Error = result.Error.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func durationMS(value time.Duration) float64 {
	if value <= 0 {
		return 0
	}
	return float64(value) / float64(time.Millisecond)
}
