package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/model"
)

func (s *Server) registerPreviews() {
	huma.Get(s.API, "/api/v1/conversations/{id}/preview", func(ctx context.Context, in *struct {
		ID  string `path:"id"`
		URL string `query:"url" maxLength:"2048"`
	}) (*Output[model.PreviewResult], error) {
		return result(s.Manager.LinkPreview(ctx, owner(ctx), in.ID, in.URL))
	})
}
