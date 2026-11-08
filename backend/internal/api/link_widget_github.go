package api

import (
	"html"
	"net/url"
	"strconv"
	"strings"
)

func githubSummaryWidget(source *url.URL, title, description, picture string) string {
	if card, ok := githubSummary(source, title); ok {
		return githubWidget(card, description, picture, source.String())
	}
	return ""
}

type githubCard struct{ repository, reference, title, icon string }

func githubSummary(u *url.URL, title string) (githubCard, bool) {
	if !strings.EqualFold(u.Hostname(), "github.com") {
		return githubCard{}, false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return githubCard{}, false
	}
	card := githubCard{repository: parts[0] + "/" + parts[1], title: title, reference: "Repository"}
	// These are the same Lucide BookMarked, CircleDot and GitPullRequest
	// paths used by the client. Icons describe resource type, never status.
	card.icon = `<path d="M10 2v7.751a.25.25 0 00.407.195l2.28-1.834a.5.5 0 01.627 0l2.28 1.834A.25.25 0 0016 9.751V2"></path><path d="M4 19.5v-15A2.5 2.5 0 016.5 2H19a1 1 0 011 1v18a1 1 0 01-1 1H6.5a1 1 0 010-5H20"></path>`
	if len(parts) == 2 {
		card.title = parts[1]
		return card, true
	}
	if len(parts) != 4 {
		return githubCard{}, false
	}
	if n, err := strconv.ParseUint(parts[3], 10, 64); err != nil || n == 0 {
		return githubCard{}, false
	}
	switch parts[2] {
	case "issues":
		card.reference = "Issue #" + parts[3]
		card.icon = `<circle cx="12" cy="12" r="1"></circle><circle cx="12" cy="12" r="10"></circle>`
		card.title = strings.TrimSuffix(title, " · "+card.reference+" · "+card.repository)
	case "pull":
		card.reference = "Pull request #" + parts[3]
		card.icon = `<circle cx="18" cy="18" r="3"></circle><circle cx="6" cy="6" r="3"></circle><path d="M13 6h3a2 2 0 0 1 2 2v7"></path><line x1="6" x2="6" y1="9" y2="21"></line>`
		card.title = strings.TrimSuffix(title, " · Pull Request #"+parts[3]+" · "+card.repository)
	default:
		return githubCard{}, false
	}
	return card, true
}

func githubWidget(card githubCard, description, picture, source string) string {
	markup := `<style>.github-card{border:1px solid color-mix(in srgb,CanvasText 16%,transparent);border-radius:12px;overflow:hidden;background:Canvas;color:CanvasText}.github-card a{display:block;color:inherit;text-decoration:none}.github-card a:focus-visible{outline:2px solid Highlight;outline-offset:-3px}.github-context{display:flex;flex-wrap:wrap;align-items:center;gap:8px 12px;padding:12px;border-bottom:1px solid color-mix(in srgb,CanvasText 12%,transparent);font-size:13px}.github-repository{flex:1;overflow-wrap:anywhere;font-weight:600;min-width:0}.github-reference{display:inline-flex;align-items:center;gap:6px;font-size:12px;white-space:nowrap}.github-card img{display:block;width:100%;height:auto;max-height:240px;object-fit:contain}.github-copy{padding:12px}.github-copy h2{font-size:16px;line-height:1.5;margin:0 0 8px;overflow-wrap:anywhere}.github-copy p{font-size:13px;line-height:1.6;margin:0 0 10px;overflow-wrap:anywhere}.github-copy small{font-size:12px}</style><article class="github-card"><a href="` + html.EscapeString(source) + `" target="_blank" rel="noopener noreferrer nofollow"><header class="github-context"><span class="github-repository">` + html.EscapeString(card.repository) + `</span><span class="github-reference"><svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + card.icon + `</svg>` + html.EscapeString(card.reference) + `</span></header>`
	if picture != "" {
		markup += `<img src="` + html.EscapeString(picture) + `" alt="" loading="lazy" referrerpolicy="no-referrer" onerror="this.remove()">`
	}
	return markup + `<div class="github-copy"><h2>` + html.EscapeString(card.title) + `</h2><p>` + html.EscapeString(description) + `</p><small>GitHub</small></div></a></article>`
}
