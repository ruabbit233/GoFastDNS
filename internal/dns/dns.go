package dns

import (
	"context"
	"time"
)

func ResolveDNS(address, domain string, attempts int, timeout time.Duration) DNSResult {
	return ResolveDNSWithOptions(address, domain, attempts, timeout, ResolveOptions{})
}

func ResolveDNSWithOptions(address, domain string, attempts int, timeout time.Duration, options ResolveOptions) DNSResult {
	return ResolveDNSWithOptionsContext(context.Background(), address, domain, attempts, timeout, options)
}

func ResolveDNSWithOptionsContext(ctx context.Context, address, domain string, attempts int, timeout time.Duration, options ResolveOptions) DNSResult {
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
		if err := ctx.Err(); err != nil {
			return DNSResult{
				Server:          address,
				Domain:          domain,
				ResolutionError: err,
				Answers:         []Answer{},
				QueryErrors:     []string{err.Error()},
				RetryCount:      i,
			}
		}
		result = resolver.Resolve(ctx, domain, timeout, options)
		result.RetryCount = i
		if result.ResolutionError == nil {
			break
		}
	}

	return result
}
