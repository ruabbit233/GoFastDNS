package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	ModeResolvePing Mode = "resolve-ping"
	ModeDNSPing     Mode = "dns-ping"
)

type Config struct {
	Mode        Mode              `yaml:"mode"`
	DNSServers  []string          `yaml:"dns_servers"`
	Domains     []string          `yaml:"domains"`
	DNS         DNSConfig         `yaml:"dns"`
	Attempts    int               `yaml:"attempts"` // 最大重试次数
	Timeout     time.Duration     `yaml:"timeout"`  // DNS 查询超时时间
	Ping        PingConfig        `yaml:"ping"`
	Benchmark   BenchmarkConfig   `yaml:"benchmark"`
	Concurrency ConcurrencyConfig `yaml:"concurrency"`
	Output      OutputConfig      `yaml:"output"`
}

type DNSConfig struct {
	RecordTypes []string `yaml:"record_types"`
}

type PingConfig struct {
	Count       int           `yaml:"count"`
	Interval    time.Duration `yaml:"interval"`
	Timeout     time.Duration `yaml:"timeout"`
	Privileged  bool          `yaml:"privileged"`
	IPSelection string        `yaml:"ip_selection"`
	IPFamily    string        `yaml:"ip_family"`
}

type BenchmarkConfig struct {
	Rounds int         `yaml:"rounds"`
	Warmup int         `yaml:"warmup"`
	Score  ScoreConfig `yaml:"score"`
}

type ScoreConfig struct {
	DNSWeight     float64 `yaml:"dns_weight"`
	PingWeight    float64 `yaml:"ping_weight"`
	SuccessWeight float64 `yaml:"success_weight"`
}

type ConcurrencyConfig struct {
	Servers int `yaml:"servers"`
	Domains int `yaml:"domains"`
	Pings   int `yaml:"pings"`
}

type OutputConfig struct {
	Format string `yaml:"format"`
	Path   string `yaml:"path"`
}

func DefaultConfig() Config {
	return Config{
		Mode:     ModeResolvePing,
		Attempts: 1,
		Timeout:  2 * time.Second,
		DNS: DNSConfig{
			RecordTypes: []string{"A"},
		},
		Ping: PingConfig{
			Count:       3,
			Interval:    100 * time.Millisecond,
			Timeout:     2 * time.Second,
			IPSelection: "all",
			IPFamily:    "ipv4",
		},
		Benchmark: BenchmarkConfig{
			Rounds: 1,
			Warmup: 0,
			Score: ScoreConfig{
				DNSWeight:     0.3,
				PingWeight:    0.6,
				SuccessWeight: 0.1,
			},
		},
		Concurrency: ConcurrencyConfig{
			Servers: 4,
			Domains: 16,
			Pings:   32,
		},
		Output: OutputConfig{
			Format: "excel",
			Path:   ".",
		},
	}
}

func LoadConfig(path string) (Config, error) {
	config := DefaultConfig()
	if path == "" {
		return config, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open config file: %w", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode config file: %w", err)
	}

	ApplyDefaults(&config)
	return config, nil
}

func ApplyDefaults(config *Config) {
	defaults := DefaultConfig()

	if config.Mode == "" {
		config.Mode = defaults.Mode
	}
	if config.Attempts <= 0 {
		config.Attempts = defaults.Attempts
	}
	if config.Timeout <= 0 {
		config.Timeout = defaults.Timeout
	}
	if len(config.DNS.RecordTypes) == 0 {
		config.DNS.RecordTypes = append([]string(nil), defaults.DNS.RecordTypes...)
	}
	config.DNS.RecordTypes = normalizeList(config.DNS.RecordTypes, strings.ToUpper)
	if config.Ping.Count <= 0 {
		config.Ping.Count = defaults.Ping.Count
	}
	if config.Ping.Interval <= 0 {
		config.Ping.Interval = defaults.Ping.Interval
	}
	if config.Ping.Timeout <= 0 {
		config.Ping.Timeout = defaults.Ping.Timeout
	}
	if config.Ping.IPSelection == "" {
		config.Ping.IPSelection = defaults.Ping.IPSelection
	}
	config.Ping.IPSelection = strings.ToLower(config.Ping.IPSelection)
	if config.Ping.IPFamily == "" {
		config.Ping.IPFamily = defaults.Ping.IPFamily
	}
	config.Ping.IPFamily = strings.ToLower(config.Ping.IPFamily)
	if config.Benchmark.Rounds == 0 &&
		config.Benchmark.Warmup == 0 &&
		config.Benchmark.Score.DNSWeight == 0 &&
		config.Benchmark.Score.PingWeight == 0 &&
		config.Benchmark.Score.SuccessWeight == 0 {
		config.Benchmark = defaults.Benchmark
	}
	if config.Concurrency.Servers == 0 &&
		config.Concurrency.Domains == 0 &&
		config.Concurrency.Pings == 0 {
		config.Concurrency = defaults.Concurrency
	}
	if config.Output.Format == "" {
		config.Output.Format = defaults.Output.Format
	}
	if config.Output.Path == "" {
		config.Output.Path = defaults.Output.Path
	}
}

func Validate(config Config) error {
	switch config.Mode {
	case ModeResolvePing, ModeDNSPing:
	default:
		return fmt.Errorf("unsupported mode %q", config.Mode)
	}

	if len(config.DNSServers) == 0 {
		return errors.New("dns_servers is required")
	}
	if config.Mode == ModeResolvePing && len(config.Domains) == 0 {
		return errors.New("domains is required for resolve-ping mode")
	}
	if config.Attempts < 0 {
		return errors.New("attempts must be greater than or equal to 0")
	}
	if config.Timeout <= 0 {
		return errors.New("timeout must be greater than 0")
	}
	if len(config.DNS.RecordTypes) == 0 {
		return errors.New("dns.record_types is required")
	}
	for _, recordType := range config.DNS.RecordTypes {
		switch strings.ToUpper(recordType) {
		case "A", "AAAA":
		default:
			return fmt.Errorf("unsupported dns.record_types value %q", recordType)
		}
	}
	if config.Ping.Count <= 0 {
		return errors.New("ping.count must be greater than 0")
	}
	if config.Ping.Interval <= 0 {
		return errors.New("ping.interval must be greater than 0")
	}
	if config.Ping.Timeout <= 0 {
		return errors.New("ping.timeout must be greater than 0")
	}
	switch config.Ping.IPSelection {
	case "all", "first":
	default:
		return fmt.Errorf("unsupported ping.ip_selection %q", config.Ping.IPSelection)
	}
	switch config.Ping.IPFamily {
	case "ipv4", "ipv6", "dual":
	default:
		return fmt.Errorf("unsupported ping.ip_family %q", config.Ping.IPFamily)
	}
	if config.Benchmark.Rounds <= 0 {
		return errors.New("benchmark.rounds must be greater than 0")
	}
	if config.Benchmark.Warmup < 0 {
		return errors.New("benchmark.warmup must be greater than or equal to 0")
	}
	if config.Benchmark.Score.DNSWeight < 0 {
		return errors.New("benchmark.score.dns_weight must be greater than or equal to 0")
	}
	if config.Benchmark.Score.PingWeight < 0 {
		return errors.New("benchmark.score.ping_weight must be greater than or equal to 0")
	}
	if config.Benchmark.Score.SuccessWeight < 0 {
		return errors.New("benchmark.score.success_weight must be greater than or equal to 0")
	}
	if config.Benchmark.Score.DNSWeight == 0 &&
		config.Benchmark.Score.PingWeight == 0 &&
		config.Benchmark.Score.SuccessWeight == 0 {
		return errors.New("at least one benchmark.score weight must be greater than 0")
	}
	if config.Concurrency.Servers <= 0 {
		return errors.New("concurrency.servers must be greater than 0")
	}
	if config.Concurrency.Domains <= 0 {
		return errors.New("concurrency.domains must be greater than 0")
	}
	if config.Concurrency.Pings <= 0 {
		return errors.New("concurrency.pings must be greater than 0")
	}

	format := strings.ToLower(config.Output.Format)
	switch format {
	case "excel", "html", "json":
	default:
		return fmt.Errorf("unsupported output format %q", config.Output.Format)
	}

	return nil
}

func normalizeList(values []string, transform func(string) string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = transform(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}
