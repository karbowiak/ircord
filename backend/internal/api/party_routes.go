package api

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"ircord/backend/internal/irc"
	"ircord/backend/internal/model"
)

type PartyPath struct {
	ID string `path:"id"`
}
type PartyCreate struct {
	Body struct {
		ClientID         string   `json:"clientId" minLength:"1" maxLength:"100"`
		Name             string   `json:"name,omitempty" maxLength:"100"`
		Description      string   `json:"description,omitempty" maxLength:"500"`
		AccountIDs       []string `json:"accountIds,omitempty" maxItems:"31" uniqueItems:"true" nullable:"false"`
		ServicesAccounts []string `json:"servicesAccounts" maxItems:"31" uniqueItems:"true" nullable:"false"`
	}
}
type PartyPatch struct {
	ID   string `path:"id"`
	Body struct {
		Name        *string `json:"name,omitempty" maxLength:"100"`
		Description *string `json:"description,omitempty" maxLength:"500"`
	}
}
type PartyAccountInput struct {
	ID   string `path:"id"`
	Body struct {
		AccountID       string `json:"accountId,omitempty" maxLength:"128"`
		ServicesAccount string `json:"servicesAccount,omitempty" maxLength:"128"`
		MemberID        string `json:"memberId,omitempty" maxLength:"128"`
	}
}

func (s *Server) registerParties() {
	huma.Get(s.API, "/api/v1/parties", func(ctx context.Context, _ *struct{}) (*Output[Items[model.Party]], error) {
		v, e := s.Manager.Parties(owner(ctx))
		return result(Items[model.Party]{Items: v}, e)
	})
	huma.Get(s.API, "/api/v1/parties/candidates", func(ctx context.Context, in *struct {
		Search string `query:"search" maxLength:"100"`
	}) (*Output[Items[model.PartyCandidate]], error) {
		v, e := s.Manager.PartyCandidatesContext(ctx, owner(ctx), in.Search)
		return result(Items[model.PartyCandidate]{Items: v}, e)
	})
	huma.Post(s.API, "/api/v1/parties", func(ctx context.Context, in *PartyCreate) (*Output[model.Party], error) {
		return nil, huma.Error410Gone("Group creation has been retired. Create an ordinary channel instead.")
	})
	huma.Post(s.API, "/api/v1/parties/{id}/convert", func(ctx context.Context, in *PartyPath) (*Output[irc.PartyConversionResult], error) {
		return result(s.Manager.ConvertParty(ctx, owner(ctx), in.ID))
	})
	huma.Get(s.API, "/api/v1/parties/{id}", func(ctx context.Context, in *PartyPath) (*Output[model.Party], error) {
		return result(s.Manager.Party(owner(ctx), in.ID))
	})
	huma.Patch(s.API, "/api/v1/parties/{id}", func(ctx context.Context, in *PartyPatch) (*Output[model.Party], error) {
		return result(s.Manager.UpdateParty(ctx, owner(ctx), in.ID, in.Body.Name, in.Body.Description))
	})
	huma.Post(s.API, "/api/v1/parties/{id}/invitations", func(ctx context.Context, in *PartyAccountInput) (*Output[model.Invitation], error) {
		if in.Body.ServicesAccount != "" {
			return result(s.Manager.InvitePartyServices(ctx, owner(ctx), in.ID, in.Body.ServicesAccount))
		}
		return result(s.Manager.InviteParty(ctx, owner(ctx), in.ID, in.Body.AccountID))
	})
	huma.Post(s.API, "/api/v1/parties/{id}/leave", func(ctx context.Context, in *PartyPath) (*Output[model.Removal], error) {
		e := s.Manager.LeaveParty(ctx, owner(ctx), in.ID)
		return result(model.Removal{ID: in.ID}, e)
	})
	huma.Delete(s.API, "/api/v1/parties/{id}/members/{memberId}", func(ctx context.Context, in *struct {
		ID       string `path:"id"`
		MemberID string `path:"memberId"`
	}) (*Output[model.Party], error) {
		return result(s.Manager.RemovePartyMember(ctx, owner(ctx), in.ID, in.MemberID))
	})
	huma.Post(s.API, "/api/v1/parties/{id}/owner", func(ctx context.Context, in *PartyAccountInput) (*Output[model.Party], error) {
		member := in.Body.MemberID
		if member == "" {
			member = in.Body.AccountID
		}
		return result(s.Manager.TransferParty(ctx, owner(ctx), in.ID, member))
	})
	huma.Post(s.API, "/api/v1/invitations/{id}/accept", func(ctx context.Context, in *PartyPath) (*Output[model.Conversation], error) {
		return result(s.Manager.AcceptInvitation(ctx, owner(ctx), in.ID))
	})
}
