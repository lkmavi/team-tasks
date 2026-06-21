package handler

import (
	"context"
	"errors"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/internal/handler/api/v1"
)

func (h *Handler) Register(ctx context.Context, req v1.RegisterRequestObject) (v1.RegisterResponseObject, error) {
	if req.Body == nil {
		return v1.Register400JSONResponse{Message: msgEmptyBody}, nil
	}
	err := h.auth.Register(ctx, string(req.Body.Email), req.Body.Name, req.Body.Password)
	switch {
	case err == nil:
		return v1.Register201Response{}, nil
	case errors.Is(err, domain.ErrConflict):
		return v1.Register409JSONResponse{Message: "email already registered"}, nil
	default:
		return nil, err
	}
}

func (h *Handler) Login(ctx context.Context, req v1.LoginRequestObject) (v1.LoginResponseObject, error) {
	if req.Body == nil {
		return v1.Login400JSONResponse{Message: msgEmptyBody}, nil
	}
	token, err := h.auth.Login(ctx, string(req.Body.Email), req.Body.Password)
	switch {
	case err == nil:
		return v1.Login200JSONResponse{Token: token}, nil
	case errors.Is(err, domain.ErrUnauthorized):
		return v1.Login401JSONResponse{Message: "invalid credentials"}, nil
	default:
		return nil, err
	}
}
