package linkmedia

import "context"

// ResolveWidget is a deprecated alias. Legacy clients cannot select a separate
// extraction path; Iframely DTOs remain only for the inactive social fetchers.
func (r *Resolver) ResolveWidget(ctx context.Context, raw string) (Result, error) {
	return r.ResolveSource(ctx, raw)
}
