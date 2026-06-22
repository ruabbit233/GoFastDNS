package cli

import (
	"GoFastDNS/internal/benchmark"
	"GoFastDNS/internal/config"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type flagOptions struct {
	rawFlags    []string
	configPath  string
	mode        string
	dnsServers  string
	domains     string
	recordTypes string
	attempts    int
	timeout     time.Duration
	pingCount   int
	pingIntv    time.Duration
	pingTime    time.Duration
	pingPriv    bool
	ipSelect    string
	ipFamily    string
	rounds      int
	warmup      int
	dnsWeight   float64
	pingWeight  float64
	succWeight  float64
	concServers int
	concDomains int
	concPings   int
	geoEnabled  bool
	geoProvider string
	geoDB       string
	geoASNDB    string
	outputPath  string
	outputFmt   string
}

func Run(args []string) int {
	return RunWithWriters(args, os.Stdout, os.Stderr)
}

func RunWithWriters(args []string, stdout, stderr io.Writer) int {
	logger := log.New(stderr, "", log.LstdFlags)

	opts, err := parseFlags(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "参数错误: %v\n", err)
		return 2
	}

	cfg, err := loadConfig(opts, args)
	if err != nil {
		fmt.Fprintf(stderr, "配置错误: %v\n", err)
		return 1
	}

	applyFlagOverrides(&cfg, opts)
	config.ApplyDefaults(&cfg)
	if err := config.Validate(cfg); err != nil {
		fmt.Fprintf(stderr, "配置错误: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	filename, err := benchmark.RunContext(ctx, cfg, logger)
	if err != nil {
		fmt.Fprintf(stderr, "运行失败: %v\n", err)
		return 1
	}
	if filename != "" {
		fmt.Fprintf(stdout, "结果已保存到 %s\n", filename)
	}
	return 0
}

func loadConfig(opts flagOptions, args []string) (config.Config, error) {
	cfg, err := config.LoadConfig(opts.configPath)
	if err == nil {
		return cfg, nil
	}

	if opts.configPath == "config.yaml" && hasOperationalFlags(args) && errors.Is(err, os.ErrNotExist) {
		return config.DefaultConfig(), nil
	}

	return config.Config{}, err
}

func parseFlags(args []string, output io.Writer) (flagOptions, error) {
	var opts flagOptions
	fs := flag.NewFlagSet("gofastdns", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.StringVar(&opts.configPath, "c", "config.yaml", "配置文件路径")
	fs.StringVar(&opts.configPath, "config", "config.yaml", "配置文件路径")
	fs.StringVar(&opts.mode, "mode", "", "运行模式: resolve-ping 或 dns-ping")
	fs.StringVar(&opts.dnsServers, "dns", "", "DNS 服务器列表，逗号分隔")
	fs.StringVar(&opts.domains, "domains", "", "域名列表，逗号分隔")
	fs.StringVar(&opts.recordTypes, "record-types", "", "DNS 记录类型列表，逗号分隔: A 或 AAAA")
	fs.IntVar(&opts.attempts, "attempts", -1, "DNS 查询失败后的最大重试次数")
	fs.DurationVar(&opts.timeout, "timeout", 0, "DNS 查询超时时间，如 1s")
	fs.IntVar(&opts.pingCount, "ping-count", -1, "每个 IP 的 ping 次数")
	fs.DurationVar(&opts.pingIntv, "ping-interval", 0, "ping 间隔，如 100ms")
	fs.DurationVar(&opts.pingTime, "ping-timeout", 0, "ping 超时时间，如 2s")
	fs.BoolVar(&opts.pingPriv, "ping-privileged", false, "使用特权 raw ICMP ping")
	fs.StringVar(&opts.ipSelect, "ip-selection", "", "解析 IP 的 ping 目标选择策略: all 或 first")
	fs.StringVar(&opts.ipFamily, "ip-family", "", "Ping IP family: ipv4、ipv6 或 dual")
	fs.IntVar(&opts.rounds, "rounds", -1, "正式 benchmark 轮数")
	fs.IntVar(&opts.warmup, "warmup", -1, "不计入统计的预热轮数")
	fs.Float64Var(&opts.dnsWeight, "score-dns-weight", -1, "综合评分 DNS 延迟权重")
	fs.Float64Var(&opts.pingWeight, "score-ping-weight", -1, "综合评分 Ping 延迟权重")
	fs.Float64Var(&opts.succWeight, "score-success-weight", -1, "综合评分成功率权重")
	fs.IntVar(&opts.concServers, "concurrency-servers", -1, "DNS 服务器并发数")
	fs.IntVar(&opts.concDomains, "concurrency-domains", -1, "每个 DNS 服务器内域名并发数")
	fs.IntVar(&opts.concPings, "concurrency-pings", -1, "Ping 目标全局并发数")
	fs.BoolVar(&opts.geoEnabled, "geoip-enabled", false, "启用 GeoIP/ASN 标注")
	fs.StringVar(&opts.geoProvider, "geoip-provider", "", "GeoIP 数据来源，目前支持 ip2location")
	fs.StringVar(&opts.geoDB, "geoip-db", "", "IP2Location 地理位置 BIN 数据库路径")
	fs.StringVar(&opts.geoASNDB, "geoip-asn-db", "", "IP2Location ASN BIN 数据库路径")
	fs.StringVar(&opts.outputPath, "output", "", "输出目录或文件路径")
	fs.StringVar(&opts.outputFmt, "output-format", "", "输出格式，目前支持 excel、html 或 json")

	err := fs.Parse(args)
	opts.rawFlags = args
	return opts, err
}

func applyFlagOverrides(cfg *config.Config, opts flagOptions) {
	if opts.mode != "" {
		cfg.Mode = config.Mode(opts.mode)
	}
	if opts.dnsServers != "" {
		cfg.DNSServers = splitList(opts.dnsServers)
	}
	if opts.domains != "" {
		cfg.Domains = splitList(opts.domains)
	}
	if opts.recordTypes != "" {
		cfg.DNS.RecordTypes = splitList(opts.recordTypes)
	}
	if opts.attempts >= 0 {
		cfg.Attempts = opts.attempts
	}
	if opts.timeout > 0 {
		cfg.Timeout = opts.timeout
	}
	if opts.pingCount > 0 {
		cfg.Ping.Count = opts.pingCount
	}
	if opts.pingIntv > 0 {
		cfg.Ping.Interval = opts.pingIntv
	}
	if opts.pingTime > 0 {
		cfg.Ping.Timeout = opts.pingTime
	}
	if flagWasSet(opts.rawFlags, "ping-privileged") {
		cfg.Ping.Privileged = opts.pingPriv
	}
	if opts.ipSelect != "" {
		cfg.Ping.IPSelection = opts.ipSelect
	}
	if opts.ipFamily != "" {
		cfg.Ping.IPFamily = opts.ipFamily
	}
	if flagWasSet(opts.rawFlags, "rounds") {
		cfg.Benchmark.Rounds = opts.rounds
	}
	if flagWasSet(opts.rawFlags, "warmup") {
		cfg.Benchmark.Warmup = opts.warmup
	}
	if flagWasSet(opts.rawFlags, "score-dns-weight") {
		cfg.Benchmark.Score.DNSWeight = opts.dnsWeight
	}
	if flagWasSet(opts.rawFlags, "score-ping-weight") {
		cfg.Benchmark.Score.PingWeight = opts.pingWeight
	}
	if flagWasSet(opts.rawFlags, "score-success-weight") {
		cfg.Benchmark.Score.SuccessWeight = opts.succWeight
	}
	if flagWasSet(opts.rawFlags, "concurrency-servers") {
		cfg.Concurrency.Servers = opts.concServers
	}
	if flagWasSet(opts.rawFlags, "concurrency-domains") {
		cfg.Concurrency.Domains = opts.concDomains
	}
	if flagWasSet(opts.rawFlags, "concurrency-pings") {
		cfg.Concurrency.Pings = opts.concPings
	}
	if flagWasSet(opts.rawFlags, "geoip-enabled") {
		cfg.GeoIP.Enabled = opts.geoEnabled
	}
	if opts.geoProvider != "" {
		cfg.GeoIP.Provider = opts.geoProvider
	}
	if opts.geoDB != "" {
		cfg.GeoIP.DatabasePath = opts.geoDB
	}
	if opts.geoASNDB != "" {
		cfg.GeoIP.ASNDatabasePath = opts.geoASNDB
	}
	if opts.outputPath != "" {
		cfg.Output.Path = opts.outputPath
	}
	if opts.outputFmt != "" {
		cfg.Output.Format = opts.outputFmt
	}
}

func splitList(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func hasOperationalFlags(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "-help" || arg == "--help" {
			continue
		}
		return true
	}
	return false
}

func flagWasSet(args []string, name string) bool {
	short := "-" + name
	long := "--" + name
	for _, arg := range args {
		if arg == short || arg == long {
			return true
		}
		if strings.HasPrefix(arg, short+"=") || strings.HasPrefix(arg, long+"=") {
			return true
		}
	}
	return false
}
