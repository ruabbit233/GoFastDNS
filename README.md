# GoFastDNS

## 简介

GoFastDNS 是一个使用 Go 编写的 DNS 性能测试 CLI 工具。它可以通过命令行参数或 `config.yaml` 配置运行，并将测试结果导出为 HTML 对比报告、Excel 文件或 JSON 文件。

## 功能

- `dns-ping`: 直接 ping 不同 DNS 服务器，对比请求发起者到 DNS 节点的网络延迟。
- `resolve-ping`: 使用不同 DNS 解析同一批域名，再 ping 解析出的 IP，用于比较不同 DNS 对 CDN 解析结果是否更贴近请求发起者。
- 支持 UDP、TCP、DoT DNS 地址格式，例如 `udp://8.8.8.8`、`tcp://8.8.8.8`、`tls://dns.google`。
- 支持 A、AAAA 记录查询，报告中保留结构化 DNS answers、CNAME 链、TTL、实际 Ping 目标和错误信息。
- 支持 JSON 输出，便于自动化分析或接入其他系统。
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
-output          输出目录或输出文件路径，按格式支持 .html / .xlsx / .json
-output-format   输出格式，目前支持 html、excel 或 json
```

`output.format` 可以设置为 `html`、`excel` 或 `json`。`html` 报告会生成离线可打开的单文件，包含 DNS 排名卡片、延迟条形对比、成功率、重试次数、解析结果、实际 Ping 目标和错误明细；`excel` 会生成表格文件；`json` 会输出汇总指标、域名明细、结构化 DNS answers、response codes、ping targets、ping 结果和错误信息。

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
