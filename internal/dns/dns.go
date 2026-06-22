package dns

import (
	"time"
)

func ResolveDNS(address, domain string, attempts int, timeout time.Duration) DNSResult {
	return ResolveDNSWithOptions(address, domain, attempts, timeout, ResolveOptions{})
}

func ResolveDNSWithOptions(address, domain string, attempts int, timeout time.Duration, options ResolveOptions) DNSResult {
	resolver, err := NewResolver(address)
	if err != nil {
		return DNSResult{
			Server:          address,
			Domain:          domain,
			ResolutionError: err,
			Answers:         []Answer{},
			QueryErrors:     []string{err.Error()},
		}
	}

	var result DNSResult
	for i := 0; i <= attempts; i++ {
		result = resolver.Resolve(domain, timeout, options)
		result.RetryCount = i
		if result.ResolutionError == nil {
			break
		}
	}

	return result
}
