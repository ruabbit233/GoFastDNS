package benchmark

import (
	"GoFastDNS/internal/ping"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func SaveResultsToExcel(servers []string, results []BenchmarkResult, outputPath string) (string, error) {
	f := excelize.NewFile()
	sheet := "DNS测试结果"
	f.SetSheetName("Sheet1", sheet)

	// 计算每个服务器结果需要的列数
	columnsPerServer := 9

	// 写入标题行
	for i, result := range results {
		baseCol := i * columnsPerServer // 每个服务器的起始列

		// 写入服务器标题
		serverCol := getColumnName(baseCol)
		f.SetCellValue(sheet, fmt.Sprintf("%s1", serverCol),
			fmt.Sprintf("DNS服务器 #%d: %s", i+1, result.Server))

		// 写入汇总信息表头
		f.SetCellValue(sheet, fmt.Sprintf("%s2", serverCol), "综合分")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+1)), "DNS平均(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+2)), "DNS p50(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+3)), "DNS p95(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+4)), "Ping平均(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+5)), "Ping p50(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+6)), "Ping p95(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+7)), "DNS成功率(%)")
		f.SetCellValue(sheet, fmt.Sprintf("%s2", getColumnName(baseCol+8)), "Ping成功率(%)")

		// 写入汇总数据
		f.SetCellValue(sheet, fmt.Sprintf("%s3", serverCol), result.Score)
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+1)), durationMS(result.AvgResponseTime))
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+2)), durationMS(result.DNSStats.Median))
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+3)), durationMS(result.DNSStats.P95))
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+4)), durationMS(result.AvgPingRTT))
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+5)), durationMS(result.PingStats.Median))
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+6)), durationMS(result.PingStats.P95))
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+7)), result.DNSSuccessRate*100)
		f.SetCellValue(sheet, fmt.Sprintf("%s3", getColumnName(baseCol+8)), result.PingSuccessRate*100)

		// 写入详情表头
		f.SetCellValue(sheet, fmt.Sprintf("%s5", serverCol), "轮次")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+1)), "域名")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+2)), "响应时间(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+3)), "重试次数")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+4)), "错误信息")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+5)), "解析结果")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+6)), "Ping目标")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+7)), "平均延迟(ms)")
		f.SetCellValue(sheet, fmt.Sprintf("%s5", getColumnName(baseCol+8)), "Ping错误")

		// 写入域名测试详情
		for rowIdx, domain := range result.DomainResults {
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", serverCol, rowIdx+6), domain.Round)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+1), rowIdx+6),
				domain.Domain)
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+2), rowIdx+6),
				durationMS(domain.ResponseTime))
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+3), rowIdx+6),
				domain.RetryCount)
			errorMessages := make([]string, 0)
			if domain.Error != nil {
				errorMessages = append(errorMessages, domain.Error.Error())
			}
			if domain.NoAnswer {
				errorMessages = append(errorMessages, "无可 Ping 的 A/AAAA 记录")
			}
			if len(errorMessages) > 0 {
				f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+4), rowIdx+6),
					strings.Join(errorMessages, "\n"))
			}
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+5), rowIdx+6),
				strings.Join(answerLabels(domain.Answers), "\n"))
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+6), rowIdx+6),
				pingTargetLabels(domain.DnsPingResults.PingResults))
			f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+7), rowIdx+6),
				durationMS(domain.DnsPingResults.AvgRTT))
			pingErrors := pingErrorMessages(domain.DnsPingResults.PingResults)
			if len(pingErrors) > 0 {
				f.SetCellValue(sheet, fmt.Sprintf("%s%d", getColumnName(baseCol+8), rowIdx+6), pingErrors)
			}
		}

		// 设置列宽
		for j := 0; j < columnsPerServer; j++ {
			col := getColumnName(baseCol + j)
			width := 15.0
			if j == 1 { // 域名列
				width = 40.0
			}
			f.SetColWidth(sheet, col, col, width)
		}
	}

	// 保存文件
	filename := outputFilename(outputPath, "resolve_ping_benchmark", "xlsx")
	if err := f.SaveAs(filename); err != nil {
		return "", fmt.Errorf("save excel file: %w", err)
	}

	return filename, nil
}

func pingErrorMessages(results []ping.PingResult) []string {
	messages := make([]string, 0)
	for _, result := range results {
		if err := result.FailureError(); err != nil {
			messages = append(messages, fmt.Sprintf("%s: %v", result.IP, err))
		}
	}
	return messages
}

func pingTargets(results []ping.PingResult) []string {
	if results == nil {
		return []string{}
	}
	targets := make([]string, 0, len(results))
	for _, result := range results {
		targets = append(targets, result.IP)
	}
	return targets
}

func pingTargetLabels(results []ping.PingResult) []string {
	if results == nil {
		return []string{}
	}
	labels := make([]string, 0, len(results))
	for _, result := range results {
		label := result.IP
		if geo := geoSummary(result.GeoIP); geo != "" {
			label = fmt.Sprintf("%s (%s)", label, geo)
		}
		labels = append(labels, label)
	}
	return labels
}

func SaveDNSPingResultsToExcel(results []DNSPingBenchmarkResult, outputPath string) (string, error) {
	f := excelize.NewFile()
	sheet := "DNS Ping测试结果"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"排名", "DNS服务器", "Ping目标", "目标地理/ASN", "综合分", "平均延迟(ms)", "p50(ms)", "p95(ms)", "成功率(%)", "丢包率(%)", "发送包数", "错误信息"}
	for i, header := range headers {
		col := getColumnName(i)
		f.SetCellValue(sheet, fmt.Sprintf("%s1", col), header)
		f.SetColWidth(sheet, col, col, 18)
	}
	f.SetColWidth(sheet, "B", "C", 32)

	for i, result := range results {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), result.Server)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), result.Target)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), geoSummary(result.TargetGeoIP))
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), result.Score)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), durationMS(result.RTT))
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), durationMS(result.Stats.Median))
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), durationMS(result.Stats.P95))
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), result.SuccessRate*100)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", row), result.PacketLoss)
		f.SetCellValue(sheet, fmt.Sprintf("K%d", row), result.PacketsSent)
		if result.Error != nil {
			f.SetCellValue(sheet, fmt.Sprintf("L%d", row), result.Error.Error())
		}
	}

	filename := outputFilename(outputPath, "dns_ping_benchmark", "xlsx")
	if err := f.SaveAs(filename); err != nil {
		return "", fmt.Errorf("save excel file: %w", err)
	}

	return filename, nil
}

// 将列索引转换为Excel列名（A, B, C, ..., Z, AA, AB, ...）
func getColumnName(index int) string {
	name := ""
	for index >= 0 {
		name = string(rune('A'+(index%26))) + name
		index = index/26 - 1
	}
	return name
}

func outputFilename(outputPath, prefix, extension string) string {
	extension = strings.TrimPrefix(strings.ToLower(extension), ".")
	filename := fmt.Sprintf("%s_%s.%s", prefix, time.Now().Format("20060102_150405"), extension)
	if outputPath == "" || outputPath == "." {
		return filename
	}

	if strings.HasSuffix(strings.ToLower(outputPath), "."+extension) {
		return outputPath
	}

	_ = os.MkdirAll(outputPath, 0755)
	return filepath.Join(outputPath, filename)
}
