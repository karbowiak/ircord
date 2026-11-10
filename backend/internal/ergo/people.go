package ergo

import (
	"context"
	"errors"
	"sort"
)

// RegisteredNames is a bounded, names-only projection of the privileged directory.
// Registration dates, channel lists and session data never enter its result.
func (c *Client) RegisteredNames(ctx context.Context, limit int) ([]string, bool, error) {
	limit = max(1, min(limit, 4096))
	var wire struct {
		Success  bool
		Accounts []struct {
			Success     bool
			AccountName string
		}
	}
	if err := c.call(ctx, "v1/ns/list", nil, &wire); err != nil {
		return nil, false, err
	}
	if !wire.Success || wire.Accounts == nil {
		return nil, false, errors.New("associated IRC server could not list accounts")
	}
	names := make([]string, 0, min(len(wire.Accounts), limit))
	for _, v := range wire.Accounts {
		if !v.Success || v.AccountName == "" {
			return nil, false, errors.New("associated IRC server returned invalid accounts")
		}
		if len(names) < limit {
			names = append(names, c.sanitize(v.AccountName))
		}
	}
	sort.Strings(names)
	return names, len(wire.Accounts) > limit, nil
}
