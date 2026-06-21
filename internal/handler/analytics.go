package handler

import (
	"context"

	"github.com/lkmavi/team-tasks/internal/handler/api/v1"
	"github.com/lkmavi/team-tasks/internal/middleware"
)

func (h *Handler) GetTeamSummaries(ctx context.Context, _ v1.GetTeamSummariesRequestObject) (v1.GetTeamSummariesResponseObject, error) {
	if _, ok := middleware.UserIDFromCtx(ctx); !ok {
		return v1.GetTeamSummaries401JSONResponse{Message: msgUnauthorized}, nil
	}
	result, err := h.analytics.TeamSummaries(ctx)
	if err != nil {
		return nil, err
	}
	out := make(v1.GetTeamSummaries200JSONResponse, len(result))
	for i, s := range result {
		out[i] = v1.TeamSummary{
			TeamId:        s.TeamID,
			Name:          s.Name,
			MemberCount:   s.MemberCount,
			DoneLast7Days: s.DoneLast7Days,
		}
	}
	return out, nil
}

func (h *Handler) GetTopContributors(ctx context.Context, _ v1.GetTopContributorsRequestObject) (v1.GetTopContributorsResponseObject, error) {
	if _, ok := middleware.UserIDFromCtx(ctx); !ok {
		return v1.GetTopContributors401JSONResponse{Message: msgUnauthorized}, nil
	}
	result, err := h.analytics.TopContributors(ctx)
	if err != nil {
		return nil, err
	}
	out := make(v1.GetTopContributors200JSONResponse, len(result))
	for i, c := range result {
		out[i] = v1.TopContributor{
			TeamId:     c.TeamID,
			UserId:     c.UserID,
			TaskCount:  c.TaskCount,
			RankInTeam: c.RankInTeam,
		}
	}
	return out, nil
}

func (h *Handler) GetOrphanTasks(ctx context.Context, _ v1.GetOrphanTasksRequestObject) (v1.GetOrphanTasksResponseObject, error) {
	if _, ok := middleware.UserIDFromCtx(ctx); !ok {
		return v1.GetOrphanTasks401JSONResponse{Message: msgUnauthorized}, nil
	}
	result, err := h.analytics.OrphanTasks(ctx)
	if err != nil {
		return nil, err
	}
	out := make(v1.GetOrphanTasks200JSONResponse, len(result))
	for i, t := range result {
		out[i] = v1.OrphanTask{
			Id:         t.ID,
			TeamId:     t.TeamID,
			AssigneeId: t.AssigneeID,
			Title:      t.Title,
		}
	}
	return out, nil
}
