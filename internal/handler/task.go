package handler

import (
	"context"
	"errors"

	openapiTypes "github.com/oapi-codegen/runtime/types"

	"github.com/lkmavi/team-tasks/internal/domain"
	"github.com/lkmavi/team-tasks/internal/handler/api/v1"
	"github.com/lkmavi/team-tasks/internal/middleware"
)

func (h *Handler) CreateTask(ctx context.Context, req v1.CreateTaskRequestObject) (v1.CreateTaskResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.CreateTask401JSONResponse{Message: msgUnauthorized}, nil
	}
	if req.Body == nil {
		return v1.CreateTask400JSONResponse{Message: msgEmptyBody}, nil
	}

	input := domain.CreateTaskInput{
		TeamID:      req.Body.TeamId,
		Title:       req.Body.Title,
		Description: req.Body.Description,
	}
	if req.Body.AssigneeId != nil {
		id := *req.Body.AssigneeId
		input.AssigneeID = &id
	}
	if req.Body.Priority != nil {
		input.Priority = domain.TaskPriority(*req.Body.Priority)
	}
	if req.Body.DueDate != nil {
		t := req.Body.DueDate.Time
		input.DueDate = &t
	}

	task, err := h.tasks.Create(ctx, userID, input)
	switch {
	case err == nil:
		return v1.CreateTask201JSONResponse(toAPITask(task)), nil
	case errors.Is(err, domain.ErrForbidden):
		return v1.CreateTask403JSONResponse{Message: "not a team member"}, nil
	default:
		return nil, err
	}
}

func (h *Handler) ListTasks(ctx context.Context, req v1.ListTasksRequestObject) (v1.ListTasksResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.ListTasks401JSONResponse{Message: msgUnauthorized}, nil
	}

	filter := domain.TaskFilter{}
	if req.Params.TeamId != nil {
		id := *req.Params.TeamId
		filter.TeamID = &id
	}
	if req.Params.Status != nil {
		s := domain.TaskStatus(*req.Params.Status)
		filter.Status = &s
	}
	if req.Params.AssigneeId != nil {
		id := *req.Params.AssigneeId
		filter.AssigneeID = &id
	}
	if req.Params.Page != nil {
		filter.Page = *req.Params.Page
	}
	if req.Params.Limit != nil {
		filter.Limit = *req.Params.Limit
	}

	tasks, total, err := h.tasks.List(ctx, userID, filter)
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrForbidden):
		return v1.ListTasks403JSONResponse{Message: msgForbidden}, nil
	default:
		return nil, err
	}

	apiTasks := make([]v1.Task, len(tasks))
	for i := range tasks {
		apiTasks[i] = toAPITask(tasks[i])
	}
	return v1.ListTasks200JSONResponse{
		Tasks: apiTasks,
		Total: total,
		Page:  filter.Page,
		Limit: filter.Limit,
	}, nil
}

func (h *Handler) UpdateTask(ctx context.Context, req v1.UpdateTaskRequestObject) (v1.UpdateTaskResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.UpdateTask401JSONResponse{Message: msgUnauthorized}, nil
	}
	if req.Body == nil {
		return v1.UpdateTask400JSONResponse{Message: msgEmptyBody}, nil
	}

	input := domain.UpdateTaskInput{
		Title:       req.Body.Title,
		Description: req.Body.Description,
	}
	if req.Body.AssigneeId != nil {
		id := *req.Body.AssigneeId
		input.AssigneeID = &id
	}
	if req.Body.Status != nil {
		s := domain.TaskStatus(*req.Body.Status)
		input.Status = &s
	}
	if req.Body.Priority != nil {
		p := domain.TaskPriority(*req.Body.Priority)
		input.Priority = &p
	}
	if req.Body.DueDate != nil {
		t := req.Body.DueDate.Time
		input.DueDate = &t
	}

	task, err := h.tasks.Update(ctx, userID, req.Id, input)
	switch {
	case err == nil:
		return v1.UpdateTask200JSONResponse(toAPITask(task)), nil
	case errors.Is(err, domain.ErrNotFound):
		return v1.UpdateTask404JSONResponse{Message: msgTaskNotFound}, nil
	case errors.Is(err, domain.ErrForbidden):
		return v1.UpdateTask403JSONResponse{Message: msgForbidden}, nil
	default:
		return nil, err
	}
}

func (h *Handler) GetTaskHistory(ctx context.Context, req v1.GetTaskHistoryRequestObject) (v1.GetTaskHistoryResponseObject, error) {
	userID, ok := middleware.UserIDFromCtx(ctx)
	if !ok {
		return v1.GetTaskHistory401JSONResponse{Message: msgUnauthorized}, nil
	}

	history, err := h.tasks.History(ctx, userID, req.Id)
	switch {
	case err == nil:
		items := make([]v1.TaskHistory, len(history))
		for i, e := range history {
			items[i] = v1.TaskHistory{
				Id:        e.ID,
				TaskId:    e.TaskID,
				ChangedBy: e.ChangedBy,
				Field:     e.Field,
				OldValue:  e.OldValue,
				NewValue:  e.NewValue,
				ChangedAt: e.ChangedAt,
			}
		}
		return v1.GetTaskHistory200JSONResponse{History: items}, nil
	case errors.Is(err, domain.ErrNotFound):
		return v1.GetTaskHistory404JSONResponse{Message: msgTaskNotFound}, nil
	case errors.Is(err, domain.ErrForbidden):
		return v1.GetTaskHistory403JSONResponse{Message: msgForbidden}, nil
	default:
		return nil, err
	}
}

func toAPITask(t domain.Task) v1.Task {
	task := v1.Task{
		Id:        t.ID,
		TeamId:    t.TeamID,
		CreatedBy: t.CreatedBy,
		Title:     t.Title,
		Status:    v1.TaskStatus(t.Status),
		Priority:  v1.TaskPriority(t.Priority),
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.AssigneeID != nil {
		id := *t.AssigneeID
		task.AssigneeId = &id
	}
	task.Description = t.Description
	if t.DueDate != nil {
		d := openapiTypes.Date{Time: *t.DueDate}
		task.DueDate = &d
	}
	return task
}
