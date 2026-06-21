package handler

import (
	"context"
	"errors"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/internal/handler/api/v1"
	"github.com/lkmavi/team-tasks/internal/middleware"
)

func (h *Handler) ListTaskComments(ctx context.Context, req v1.ListTaskCommentsRequestObject) (v1.ListTaskCommentsResponseObject, error) {
	callerID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.ListTaskComments401JSONResponse{Message: msgUnauthorized}, nil
	}

	comments, err := h.comments.List(ctx, callerID, req.Id)
	switch {
	case err == nil:
		out := make(v1.ListTaskComments200JSONResponse, len(comments))
		for i := range comments {
			out[i] = toAPIComment(comments[i])
		}
		return out, nil
	case errors.Is(err, domain.ErrNotFound):
		return v1.ListTaskComments404JSONResponse{Message: msgTaskNotFound}, nil
	case errors.Is(err, domain.ErrForbidden):
		return v1.ListTaskComments403JSONResponse{Message: msgForbidden}, nil
	default:
		return nil, err
	}
}

func (h *Handler) AddTaskComment(ctx context.Context, req v1.AddTaskCommentRequestObject) (v1.AddTaskCommentResponseObject, error) {
	callerID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.AddTaskComment401JSONResponse{Message: msgUnauthorized}, nil
	}
	if req.Body == nil || req.Body.Body == "" {
		return v1.AddTaskComment400JSONResponse{Message: "body is required"}, nil
	}

	c, err := h.comments.Add(ctx, callerID, req.Id, req.Body.Body)
	switch {
	case err == nil:
		return v1.AddTaskComment201JSONResponse(toAPIComment(c)), nil
	case errors.Is(err, domain.ErrNotFound):
		return v1.AddTaskComment404JSONResponse{Message: msgTaskNotFound}, nil
	case errors.Is(err, domain.ErrForbidden):
		return v1.AddTaskComment403JSONResponse{Message: msgForbidden}, nil
	default:
		return nil, err
	}
}

func toAPIComment(c domain.Comment) v1.Comment {
	return v1.Comment{
		Id:        c.ID,
		TaskId:    c.TaskID,
		UserId:    c.UserID,
		Body:      c.Body,
		CreatedAt: c.CreatedAt,
	}
}
