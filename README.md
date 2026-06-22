# GoFastDNS

## 简介

GoFastDNS 是一个使用 Go 编写的 DNS 性能测试 CLI 工具。它可以通过命令行参数或 `config.yaml` 配置运行，并将测试结果导出为 HTML 对比报告、Excel 文件或 JSON 文件。

## 功能

- `dns-ping`: 直接 ping 不同 DNS 服务器，对比请求发起者到 DNS 节点的网络延迟。
- `resolve-ping`: 使用不同 DNS 解析同一批域名，再 ping 解析出的 IP，用于比较不同 DNS 对 CDN 解析结果是否更贴近请求发起者。
- 支持 UDP、TCP、DoT 和 DoH DNS 地址格式，例如 `udp://8.8.8.8`、`tcp://8.8.8.8`、`tls://dns.google`、`https://dns.google/dns-query`。
- 支持 A、AAAA 记录查询，报告中保留结构化 DNS answers、CNAME 链、TTL、实际 Ping 目标和错误信息。
- 支持 JSON 输出，便于自动化分析或接入其他系统。
- 支持多轮测试、p50/p95/jitter、DNS/Ping 成功率、综合评分和可配置并发。
- 可选 GeoIP / ASN 标注，使用 IP2Location BIN 数据库解释解析 IP 的地理位置和网络归属。
- 支持通过 `-c config.yaml` 读取配置，并用命令行参数覆盖配置文件字段。

## 安装与运行

1. 确保已在环境中安装 Go。
2. 将项目克隆到本地后，进入项目根目录。
3. 通过配置文件运行:

```bash
go run . -c config.yaml
```

也可以直接使用命令行参数运行:

```bash
go run . -mode dns-ping -dns udp://8.8.8.8,udp://1.1.1.1
```

```bash
go run . -mode resolve-ping -dns udp://8.8.8.8,tls://dns.google -domains baidu.com,bilibili.com
```

命令行参数优先级高于配置文件:

```bash
go run . -c config.yaml -mode dns-ping -dns udp://223.5.5.5,udp://119.29.29.29
```

## 配置示例

```yaml
mode: resolve-ping
attempts: 2
timeout: 1s

dns:
  record_types:
    - A

benchmark:
  rounds: 3
  warmup: 1
  score:
    dns_weight: 0.3
    ping_weight: 0.6
    success_weight: 0.1

concurrency:
  servers: 4
  domains: 16
  pings: 32

geoip:
  enabled: false
  provider: ip2location
  database_path: ./IP2LOCATION-LITE-DB11.BIN
  asn_database_path: ./IP2LOCATION-LITE-ASN.BIN

ping:
  count: 3
  interval: 100ms
  timeout: 2s
  privileged: false
  ip_selection: all
  ip_family: ipv4

output:
  format: html
  path: .

dns_servers:
  - "udp://8.8.8.8"
  - "tls://dns.google"
  - "https://dns.google/dns-query"

domains:
  - "baidu.com"
  - "bilibili.com"
```

## 参数

```text
-c, -config       配置文件路径，默认 config.yaml
-mode            运行模式: resolve-ping 或 dns-ping
-dns             DNS 服务器列表，逗号分隔
-domains         域名列表，逗号分隔
-record-types    DNS 记录类型列表，逗号分隔: A 或 AAAA
-attempts        DNS 查询失败后的最大重试次数
-timeout         DNS 查询超时时间，例如 1s
-ping-count      每个 IP 的 ping 次数
-ping-interval   ping 间隔，例如 100ms
-ping-timeout    ping 超时时间，例如 2s
-ping-privileged 使用特权 raw ICMP ping
-ip-selection    解析 IP 的 ping 目标选择策略: all 或 first
-ip-family       Ping IP family: ipv4、ipv6 或 dual
-rounds          正式 benchmark 轮数
-warmup          不计入统计的预热轮数
-score-dns-weight        综合评分 DNS 延迟权重
-score-ping-weight       综合评分 Ping 延迟权重
-score-success-weight    综合评分成功率权重
-concurrency-servers     DNS 服务器并发数
-concurrency-domains     每个 DNS 服务器内域名并发数
-concurrency-pings       Ping 目标全局并发数
-geoip-enabled           启用 GeoIP/ASN 标注
-geoip-provider          GeoIP 数据来源，目前支持 ip2location
-geoip-db                IP2Location 地理位置 BIN 数据库路径
-geoip-asn-db            IP2Location ASN BIN 数据库路径
-output          输出目录或输出文件路径，按格式支持 .html / .xlsx / .json
-output-format   输出格式，目前支持 html、excel 或 json
```

`benchmark.rounds` 控制正式测试轮数，`benchmark.warmup` 控制预热轮数，预热结果不会进入统计。`rounds: 1`、`warmup: 0` 的行为接近旧版单轮测试。报告会展示平均值、p50、p95、jitter、DNS 成功率、Ping 成功率和综合分；失败 ping 不会作为 0 延迟参与统计。

`concurrency.servers` 控制 DNS 服务器并发数，`concurrency.domains` 控制每个 DNS 服务器内的域名并发数，`concurrency.pings` 控制全局 Ping 目标并发数。按下 Ctrl+C 时，程序会尽量停止启动新任务并取消正在等待的 DNS/Ping 操作。

DNS 服务器地址支持 `udp://`、`tcp://`、`tls://` 和 DoH `https://`。DoH 使用 DNS wire format over HTTPS，HTTP 状态码错误、DNS RCODE 和网络错误会分别保留在输出错误字段中:

```bash
go run . -mode resolve-ping -dns https://dns.google/dns-query,https://cloudflare-dns.com/dns-query -domains example.com
```

`geoip.enabled` 默认关闭。开启后程序会读取本地 IP2Location 数据库，为结构化 DNS answers、实际 Ping 目标以及 `dns-ping` 目标增加 `geoip` / `target_geoip` 字段，HTML 和 Excel 中会展示国家、地区、ASN、AS 名称和 ISP 摘要。数据库路径缺失或格式不支持时，程序会在启动时返回错误:

```bash
go run . -c config.yaml -geoip-enabled -output-format json
```

`output.format` 可以设置为 `html`、`excel` 或 `json`。`html` 报告会生成离线可打开的单文件，包含 DNS 排名卡片、延迟条形对比、成功率、重试次数、解析结果、实际 Ping 目标和错误明细；`excel` 会生成表格文件；`json` 会输出汇总指标、统计指标、域名明细、结构化 DNS answers、响应码、Ping 目标、Ping 结果和错误信息。

```bash
go run . -c config.yaml -output-format html -output results
```

```bash
go run . -c config.yaml -output-format json -output results
```

`ping.privileged` 默认是 `false`，使用非特权 ping。若运行环境要求 raw ICMP 权限，可以在配置文件中改为 `true`，并用管理员权限运行；如果解析后的 IP 全部 ping 失败，日志和输出报告的错误列会显示失败原因，平均延迟不会再被误解为有效的 0 延迟。

`ping.ip_selection` 默认是 `all`，会 ping DNS 返回的所有匹配 IP 并计算平均延迟。设置为 `first` 时，只 ping DNS 响应里的第一个匹配 IP，用于近似模拟常见客户端优先尝试首个候选地址的行为:

```bash
go run . -mode resolve-ping -dns udp://8.8.8.8,tls://dns.google -domains baidu.com,bilibili.com -ip-selection first
```

默认配置只查询 A 记录并只 ping IPv4 地址，以保持旧行为。需要双栈测试时，可以同时启用 A/AAAA 查询并设置 `ping.ip_family`:

```yaml
dns:
  record_types:
    - A
    - AAAA
ping:
  ip_family: dual
```

也可以用命令行覆盖:

```bash
go run . -mode resolve-ping -dns udp://8.8.8.8 -domains example.com -record-types A,AAAA -ip-family dual -output-format json
```

## 测试

默认单元测试不触发真实 DNS 或 ICMP:

```bash
go test ./...
```

需要真实网络和 ping 权限的测试使用 `integration` 标签:

```bash
go test -tags=integration ./internal/ping
```

## 贡献

如有问题或改进建议，欢迎提交 Issue 或 Pull Request。
