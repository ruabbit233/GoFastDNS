package benchmark

import (
	"GoFastDNS/internal/dns"
	"fmt"
)

func answerLabels(answers []dns.Answer) []string {
	labels := make([]string, 0, len(answers))
	for _, answer := range answers {
		label := fmt.Sprintf("%s %s", answer.Type, answer.Value)
		if answer.TTL > 0 {
			label = fmt.Sprintf("%s TTL=%d", label, answer.TTL)
		}
		labels = append(labels, label)
	}
	return labels
}
