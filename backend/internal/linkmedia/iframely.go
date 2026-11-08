package linkmedia

import (
	"context"

	"ircord/backend/internal/iframely"
	"ircord/backend/internal/mediafetch"
)

// ResolveWidget also serves browser retries when a native source fails.
func (r *Resolver) ResolveWidget(ctx context.Context, raw string) (Result, error) {
	return r.cache.get(ctx, "widget:"+resolutionKey(raw), r.options.Timeout, func(ctx context.Context) (Result, error) { return r.resolveWidget(ctx, raw) })
}

func (r *Resolver) resolveWidget(ctx context.Context, raw string) (Result, error) {
	u, err := mediafetch.URL(raw)
	if err != nil {
		return Result{}, err
	}
	if r.options.Iframely == nil {
		return Result{}, ErrUnavailable
	}
	embed, err := r.options.Iframely.Lookup(ctx, raw, iframely.RequestOptions{})
	if err != nil {
		return Result{}, err
	}
	// Some plugins provide markup only on a player/app link.
	if embed.HTML == "" {
		for _, rel := range []string{"player", "app", "reader", "summary"} {
			for _, link := range embed.Links[rel] {
				if link.HTML != "" {
					embed.HTML = link.HTML
					break
				}
			}
			if embed.HTML != "" {
				break
			}
		}
	}
	if embed.HTML == "" && embed.Meta.Title == "" && len(embed.Links) == 0 {
		return Result{}, ErrUnavailable
	}
	if handler := handlerFor(u); handler != nil && handler.widget != nil {
		if result, ok := handler.widget(u, embed); ok {
			result.URL = raw
			return result, nil
		}
	}
	return Result{URL: raw, Provider: "iframely", Title: embed.Meta.Title, Description: embed.Meta.Description, Author: embed.Meta.Author, Embed: &embed}, nil
}
