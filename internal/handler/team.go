package handler

import (
	"context"
	"errors"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/internal/handler/api/v1"
	"github.com/lkmavi/team-tasks/internal/middleware"
)

func (h *Handler) ListTeams(ctx context.Context, _ v1.ListTeamsRequestObject) (v1.ListTeamsResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.ListTeams401JSONResponse{Message: msgUnauthorized}, nil
	}

	teams, err := h.teams.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	out := make([]v1.Team, len(teams))
	for i := range teams {
		out[i] = toAPITeam(teams[i])
	}
	return v1.ListTeams200JSONResponse{Teams: out}, nil
}

func (h *Handler) CreateTeam(ctx context.Context, req v1.CreateTeamRequestObject) (v1.CreateTeamResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.CreateTeam401JSONResponse{Message: msgUnauthorized}, nil
	}
	if req.Body == nil {
		return v1.CreateTeam400JSONResponse{Message: msgEmptyBody}, nil
	}

	team, err := h.teams.Create(ctx, userID, req.Body.Name)
	if err != nil {
		return nil, err
	}
	return v1.CreateTeam201JSONResponse(toAPITeam(team)), nil
}

func (h *Handler) InviteToTeam(ctx context.Context, req v1.InviteToTeamRequestObject) (v1.InviteToTeamResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.InviteToTeam401JSONResponse{Message: msgUnauthorized}, nil
	}
	if req.Body == nil {
		return v1.InviteToTeam400JSONResponse{Message: msgEmptyBody}, nil
	}

	err := h.teams.Invite(ctx, userID, req.Id, req.Body.UserId)
	switch {
	case err == nil:
		return v1.InviteToTeam200Response{}, nil
	case errors.Is(err, domain.ErrForbidden):
		return v1.InviteToTeam403JSONResponse{Message: msgForbidden}, nil
	case errors.Is(err, domain.ErrNotFound):
		return v1.InviteToTeam404JSONResponse{Message: "not found"}, nil
	default:
		return nil, err
	}
}

func toAPITeam(t domain.Team) v1.Team {
	return v1.Team{
		Id:        t.ID,
		Name:      t.Name,
		CreatedBy: t.CreatedBy,
		CreatedAt: t.CreatedAt,
	}
}
