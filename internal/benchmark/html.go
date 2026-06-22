package benchmark

import (
	"fmt"
	"html/template"
	"os"
	"time"
)

type htmlReport struct {
	Title       string
	GeneratedAt string
	ModeLabel   string
	ResolveRows []resolveHTMLRow
	DNSPingRows []dnsPingHTMLRow
}

type resolveHTMLRow struct {
	Rank            int
	Server          string
	AvgDNS          string
	AvgDNSMS        int64
	AvgPing         string
	AvgPingMS       int64
	SuccessRate     string
	SuccessRatePct  float64
	TotalRetries    int
	DNSBarWidth     float64
	PingBarWidth    float64
	SuccessBarWidth float64
	DomainRows      []domainHTMLRow
}

type domainHTMLRow struct {
	Domain       string
	ResponseTime string
	Retries      int
	Error        string
	NoAnswer     bool
	Answers      []string
	PingTargets  []string
	AvgPing      string
	PingErrors   []string
}

type dnsPingHTMLRow struct {
	Rank         int
	Server       string
	Target       string
	RTT          string
	RTTMS        int64
	RTTBarWidth  float64
	PacketLoss   string
	LossBarWidth float64
	PacketsSent  int
	Error        string
}

func SaveResultsToHTML(results []BenchmarkResult, outputPath string) (string, error) {
	report := htmlReport{
		Title:       "Resolve Ping Benchmark",
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		ModeLabel:   "resolve-ping",
		ResolveRows: buildResolveHTMLRows(results),
	}

	filename := outputFilename(outputPath, "resolve_ping_benchmark", "html")
	if err := writeHTMLReport(filename, report); err != nil {
		return "", err
	}
	return filename, nil
}

func SaveDNSPingResultsToHTML(results []DNSPingBenchmarkResult, outputPath string) (string, error) {
	report := htmlReport{
		Title:       "DNS Ping Benchmark",
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
		ModeLabel:   "dns-ping",
		DNSPingRows: buildDNSPingHTMLRows(results),
	}

	filename := outputFilename(outputPath, "dns_ping_benchmark", "html")
	if err := writeHTMLReport(filename, report); err != nil {
		return "", err
	}
	return filename, nil
}

func writeHTMLReport(filename string, report htmlReport) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create html file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if err := htmlReportTemplate.Execute(file, report); err != nil {
		return fmt.Errorf("write html file: %w", err)
	}
	return nil
}

func buildResolveHTMLRows(results []BenchmarkResult) []resolveHTMLRow {
	maxDNS := maxResolveDNSMS(results)
	maxPing := maxResolvePingMS(results)

	rows := make([]resolveHTMLRow, 0, len(results))
	for i, result := range results {
		avgDNSMS := result.AvgResponseTime.Milliseconds()
		avgPingMS := result.AvgPingRTT.Milliseconds()
		row := resolveHTMLRow{
			Rank:            i + 1,
			Server:          result.Server,
			AvgDNS:          formatDuration(result.AvgResponseTime),
			AvgDNSMS:        avgDNSMS,
			AvgPing:         formatDuration(result.AvgPingRTT),
			AvgPingMS:       avgPingMS,
			SuccessRate:     formatPercent(result.SuccessRate),
			SuccessRatePct:  result.SuccessRate * 100,
			TotalRetries:    result.TotalRetries,
			DNSBarWidth:     inverseBarWidth(avgDNSMS, maxDNS),
			PingBarWidth:    inverseBarWidth(avgPingMS, maxPing),
			SuccessBarWidth: clampPercent(result.SuccessRate * 100),
			DomainRows:      buildDomainHTMLRows(result.DomainResults),
		}
		rows = append(rows, row)
	}
	return rows
}

func buildDomainHTMLRows(results []DomainResult) []domainHTMLRow {
	rows := make([]domainHTMLRow, 0, len(results))
	for _, result := range results {
		row := domainHTMLRow{
			Domain:       result.Domain,
			ResponseTime: formatDuration(result.ResponseTime),
			Retries:      result.RetryCount,
			NoAnswer:     result.NoAnswer,
			Answers:      answerLabels(result.Answers),
			PingTargets:  pingTargets(result.DnsPingResults.PingResults),
			AvgPing:      formatDuration(result.DnsPingResults.AvgRTT),
			PingErrors:   pingErrorMessages(result.DnsPingResults.PingResults),
		}
		if result.Error != nil {
			row.Error = result.Error.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func buildDNSPingHTMLRows(results []DNSPingBenchmarkResult) []dnsPingHTMLRow {
	maxRTT := maxDNSPingRTTMS(results)

	rows := make([]dnsPingHTMLRow, 0, len(results))
	for i, result := range results {
		rttMS := result.RTT.Milliseconds()
		row := dnsPingHTMLRow{
			Rank:         i + 1,
			Server:       result.Server,
			Target:       result.Target,
			RTT:          formatDuration(result.RTT),
			RTTMS:        rttMS,
			RTTBarWidth:  inverseBarWidth(rttMS, maxRTT),
			PacketLoss:   formatPercentValue(result.PacketLoss),
			LossBarWidth: clampPercent(result.PacketLoss),
			PacketsSent:  result.PacketsSent,
		}
		if result.Error != nil {
			row.Error = result.Error.Error()
		}
		rows = append(rows, row)
	}
	return rows
}

func maxResolveDNSMS(results []BenchmarkResult) int64 {
	var max int64
	for _, result := range results {
		if value := result.AvgResponseTime.Milliseconds(); value > max {
			max = value
		}
	}
	return max
}

func maxResolvePingMS(results []BenchmarkResult) int64 {
	var max int64
	for _, result := range results {
		if value := result.AvgPingRTT.Milliseconds(); value > max {
			max = value
		}
	}
	return max
}

func maxDNSPingRTTMS(results []DNSPingBenchmarkResult) int64 {
	var max int64
	for _, result := range results {
		if result.Error != nil {
			continue
		}
		if value := result.RTT.Milliseconds(); value > max {
			max = value
		}
	}
	return max
}

func inverseBarWidth(value, max int64) float64 {
	if value <= 0 || max <= 0 {
		return 0
	}
	width := 100 - (float64(value)/float64(max))*72
	if width < 18 {
		return 18
	}
	if width > 100 {
		return 100
	}
	return width
}

func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func formatDuration(value time.Duration) string {
	if value <= 0 {
		return "-"
	}
	if value < time.Millisecond {
		return fmt.Sprintf("%.2f ms", float64(value)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%d ms", value.Milliseconds())
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value*100)
}

func formatPercentValue(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func formatBarWidth(value float64) template.CSS {
	return template.CSS(fmt.Sprintf("%.1f%%", clampPercent(value)))
}

var htmlReportTemplate = template.Must(template.New("html-report").Funcs(template.FuncMap{
	"formatBarWidth":  formatBarWidth,
	"hasResolveRows":  func(rows []resolveHTMLRow) bool { return len(rows) > 0 },
	"hasDNSPingRows":  func(rows []dnsPingHTMLRow) bool { return len(rows) > 0 },
	"hasStringValues": func(values []string) bool { return len(values) > 0 },
}).Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root {
  color-scheme: light;
  --bg: #f4f6f8;
  --surface: #ffffff;
  --ink: #18202a;
  --muted: #667085;
  --line: #d9e0e7;
  --good: #1f9d68;
  --warn: #d97706;
  --bad: #d64545;
  --dns: #2563eb;
  --ping: #10b981;
  --loss: #f97316;
  --soft: #edf2f7;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--ink);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-size: 14px;
  line-height: 1.5;
}
.page {
  max-width: 1280px;
  margin: 0 auto;
  padding: 28px 20px 48px;
}
.header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 22px;
}
h1 {
  margin: 0;
  font-size: 28px;
  line-height: 1.15;
  letter-spacing: 0;
}
.meta {
  color: var(--muted);
  text-align: right;
}
.section {
  margin-top: 18px;
}
h2 {
  margin: 0 0 12px;
  font-size: 18px;
  letter-spacing: 0;
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}
.summary-card {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 14px;
}
.summary-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: flex-start;
}
.rank {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 28px;
  padding: 0 8px;
  border-radius: 999px;
  background: var(--ink);
  color: #fff;
  font-weight: 700;
}
.server {
  min-width: 0;
  overflow-wrap: anywhere;
  font-weight: 700;
}
.metric-list {
  display: grid;
  gap: 10px;
  margin-top: 14px;
}
.metric {
  display: grid;
  grid-template-columns: minmax(92px, 1fr) minmax(72px, auto);
  gap: 8px;
  align-items: center;
}
.metric span {
  color: var(--muted);
}
.metric strong {
  text-align: right;
}
.bar {
  grid-column: 1 / -1;
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--soft);
}
.fill {
  width: var(--w);
  height: 100%;
  border-radius: inherit;
  background: var(--dns);
}
.fill.ping { background: var(--ping); }
.fill.success { background: var(--good); }
.fill.loss { background: var(--loss); }
.table-wrap {
  overflow-x: auto;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
}
table {
  width: 100%;
  min-width: 920px;
  border-collapse: collapse;
}
th, td {
  padding: 10px 12px;
  border-bottom: 1px solid var(--line);
  text-align: left;
  vertical-align: top;
}
th {
  position: sticky;
  top: 0;
  background: #f8fafc;
  color: #344054;
  font-size: 12px;
  text-transform: uppercase;
}
tr:last-child td { border-bottom: 0; }
.num { text-align: right; white-space: nowrap; }
.muted { color: var(--muted); }
.error { color: var(--bad); overflow-wrap: anywhere; }
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--soft);
  color: #344054;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.details {
  margin-top: 14px;
  display: grid;
  gap: 16px;
}
.detail-block {
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}
.detail-title {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  padding: 12px 14px;
  border-bottom: 1px solid var(--line);
  background: #f8fafc;
}
.detail-title strong {
  overflow-wrap: anywhere;
}
.status-ok { color: var(--good); }
.status-warn { color: var(--warn); }
@media (max-width: 720px) {
  .page { padding: 20px 12px 36px; }
  .header {
    display: block;
  }
  .meta {
    margin-top: 8px;
    text-align: left;
  }
  h1 { font-size: 24px; }
  .summary-grid {
    grid-template-columns: 1fr;
  }
}
</style>
</head>
<body>
<main class="page">
  <header class="header">
    <div>
      <h1>{{.Title}}</h1>
      <div class="muted">模式: {{.ModeLabel}}</div>
    </div>
    <div class="meta">生成时间<br>{{.GeneratedAt}}</div>
  </header>

  {{if hasResolveRows .ResolveRows}}
  <section class="section">
    <h2>DNS 解析与 CDN 延迟排行</h2>
    <div class="summary-grid">
      {{range .ResolveRows}}
      <article class="summary-card">
        <div class="summary-head">
          <div class="server">{{.Server}}</div>
          <div class="rank">#{{.Rank}}</div>
        </div>
        <div class="metric-list">
          <div class="metric">
            <span>解析 IP 平均延迟</span>
            <strong>{{.AvgPing}}</strong>
            <div class="bar"><div class="fill ping" style="--w: {{formatBarWidth .PingBarWidth}}"></div></div>
          </div>
          <div class="metric">
            <span>DNS 平均响应</span>
            <strong>{{.AvgDNS}}</strong>
            <div class="bar"><div class="fill" style="--w: {{formatBarWidth .DNSBarWidth}}"></div></div>
          </div>
          <div class="metric">
            <span>成功率</span>
            <strong>{{.SuccessRate}}</strong>
            <div class="bar"><div class="fill success" style="--w: {{formatBarWidth .SuccessBarWidth}}"></div></div>
          </div>
          <div class="metric">
            <span>总重试次数</span>
            <strong>{{.TotalRetries}}</strong>
          </div>
        </div>
      </article>
      {{end}}
    </div>
  </section>

  <section class="section">
    <h2>汇总对比</h2>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>排名</th>
            <th>DNS 服务器</th>
            <th class="num">解析 IP 平均延迟</th>
            <th class="num">DNS 平均响应</th>
            <th class="num">成功率</th>
            <th class="num">总重试</th>
          </tr>
        </thead>
        <tbody>
          {{range .ResolveRows}}
          <tr>
            <td>#{{.Rank}}</td>
            <td>{{.Server}}</td>
            <td class="num">{{.AvgPing}}</td>
            <td class="num">{{.AvgDNS}}</td>
            <td class="num">{{.SuccessRate}}</td>
            <td class="num">{{.TotalRetries}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </section>

  <section class="section">
    <h2>域名明细</h2>
    <div class="details">
      {{range .ResolveRows}}
      <article class="detail-block">
        <div class="detail-title">
          <strong>#{{.Rank}} {{.Server}}</strong>
          <span class="muted">解析 IP 平均延迟 {{.AvgPing}}</span>
        </div>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>域名</th>
                <th class="num">DNS 响应</th>
                <th class="num">重试</th>
                <th>解析结果</th>
                <th>Ping 目标</th>
                <th class="num">平均延迟</th>
                <th>错误</th>
              </tr>
            </thead>
            <tbody>
              {{range .DomainRows}}
              <tr>
                <td>{{.Domain}}</td>
                <td class="num">{{.ResponseTime}}</td>
                <td class="num">{{.Retries}}</td>
                <td>
                  {{if hasStringValues .Answers}}
                  <div class="chips">{{range .Answers}}<span class="chip">{{.}}</span>{{end}}</div>
                  {{else}}<span class="muted">-</span>{{end}}
                </td>
                <td>
                  {{if hasStringValues .PingTargets}}
                  <div class="chips">{{range .PingTargets}}<span class="chip">{{.}}</span>{{end}}</div>
                  {{else}}<span class="muted">-</span>{{end}}
                </td>
                <td class="num">{{.AvgPing}}</td>
                <td>
                  {{if .Error}}<div class="error">{{.Error}}</div>{{end}}
                  {{if .NoAnswer}}<div class="status-warn">无可 Ping 的 A/AAAA 记录</div>{{end}}
                  {{if hasStringValues .PingErrors}}
                  {{range .PingErrors}}<div class="error">{{.}}</div>{{end}}
                  {{end}}
                  {{if and (not .Error) (not .NoAnswer) (not (hasStringValues .PingErrors))}}<span class="status-ok">正常</span>{{end}}
                </td>
              </tr>
              {{end}}
            </tbody>
          </table>
        </div>
      </article>
      {{end}}
    </div>
  </section>
  {{end}}

  {{if hasDNSPingRows .DNSPingRows}}
  <section class="section">
    <h2>DNS 节点 Ping 排行</h2>
    <div class="summary-grid">
      {{range .DNSPingRows}}
      <article class="summary-card">
        <div class="summary-head">
          <div>
            <div class="server">{{.Server}}</div>
            <div class="muted">目标: {{.Target}}</div>
          </div>
          <div class="rank">#{{.Rank}}</div>
        </div>
        <div class="metric-list">
          <div class="metric">
            <span>平均延迟</span>
            <strong>{{.RTT}}</strong>
            <div class="bar"><div class="fill ping" style="--w: {{formatBarWidth .RTTBarWidth}}"></div></div>
          </div>
          <div class="metric">
            <span>丢包率</span>
            <strong>{{.PacketLoss}}</strong>
            <div class="bar"><div class="fill loss" style="--w: {{formatBarWidth .LossBarWidth}}"></div></div>
          </div>
          <div class="metric">
            <span>发送包数</span>
            <strong>{{.PacketsSent}}</strong>
          </div>
          {{if .Error}}<div class="error">{{.Error}}</div>{{else}}<div class="status-ok">正常</div>{{end}}
        </div>
      </article>
      {{end}}
    </div>
  </section>

  <section class="section">
    <h2>明细对比</h2>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>排名</th>
            <th>DNS 服务器</th>
            <th>Ping 目标</th>
            <th class="num">平均延迟</th>
            <th class="num">丢包率</th>
            <th class="num">发送包数</th>
            <th>错误</th>
          </tr>
        </thead>
        <tbody>
          {{range .DNSPingRows}}
          <tr>
            <td>#{{.Rank}}</td>
            <td>{{.Server}}</td>
            <td>{{.Target}}</td>
            <td class="num">{{.RTT}}</td>
            <td class="num">{{.PacketLoss}}</td>
            <td class="num">{{.PacketsSent}}</td>
            <td>{{if .Error}}<span class="error">{{.Error}}</span>{{else}}<span class="status-ok">正常</span>{{end}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
    </div>
  </section>
  {{end}}
</main>
</body>
</html>
`))
