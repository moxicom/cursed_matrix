package httphandler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

type tasksResponse struct {
	Tasks     []gen.Task     `json:"tasks"`
	Links     []gen.TaskLink `json:"links"`
	Truncated bool           `json:"truncated"`
}

// ListTasks answers with the whole working set the filter matches.
func (a *API) ListTasks(w http.ResponseWriter, r *http.Request, params gen.ListTasksParams) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	result, err := a.board.List(r.Context(), userID, boardQuery(params))
	if err != nil {
		WriteError(w, r, err)
		return
	}

	// An empty result renders as [] rather than null: the client treats the
	// field as a list and an absent board is not an error.
	rendered := make([]gen.Task, 0, len(result.Tasks))
	for i := range result.Tasks {
		rendered = append(rendered, renderTask(&result.Tasks[i]))
	}

	// The links come with the board because a card shows how many a task has
	// and the topology filter needs them; fetching them separately would let
	// the two answers disagree.
	links, err := a.graph.Links(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	edges := make([]gen.TaskLink, 0, len(links))
	for i := range links {
		edges = append(edges, renderLink(&links[i]))
	}

	WriteJSON(w, r, http.StatusOK, tasksResponse{
		Tasks:     rendered,
		Links:     edges,
		Truncated: result.Truncated,
	})
}

// boardQuery carries the query string across the boundary unvalidated: the
// enums are the domain's to accept or refuse, and Filter.Validate says which
// field was wrong.
func boardQuery(params gen.ListTasksParams) board.Query {
	query := board.Query{Search: stringValue(params.Query)}

	if params.Status != nil {
		status := task.StatusFilter(*params.Status)
		query.Status = &status
	}
	if params.Deadline != nil {
		window := task.DeadlineWindow(*params.Deadline)
		query.Deadline = &window
	}
	if params.Topology != nil {
		topology := task.TopologyFilter(*params.Topology)
		query.Topology = &topology
	}
	if params.Sort != nil {
		field := task.SortField(*params.Sort)
		query.Sort = &field
	}
	if params.Direction != nil {
		direction := task.SortDirection(*params.Direction)
		query.Direction = &direction
	}
	if params.Quadrants != nil {
		for _, q := range *params.Quadrants {
			query.Quadrants = append(query.Quadrants, shared.Quadrant(q))
		}
	}
	if params.Colors != nil {
		for _, c := range *params.Colors {
			query.Colors = append(query.Colors, shared.TaskColor(c))
		}
	}
	if params.Tags != nil {
		query.Tags = *params.Tags
	}
	return query
}

func renderTask(t *task.Task) gen.Task {
	rendered := gen.Task{
		Id:              t.ID,
		UserId:          t.UserID,
		ParentTaskId:    t.ParentID,
		Title:           t.Title,
		Description:     t.Description,
		Position:        t.Position,
		Color:           gen.TaskColor(t.Color),
		DeadlineAt:      t.DeadlineAt,
		DeadlineHasTime: t.DeadlineHasTime,
		Status:          gen.TaskStatus(t.Status),
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
		CompletedAt:     t.CompletedAt,
		XpAwarded:       t.XPAwarded,
		Tags:            t.Tags,
	}
	if rendered.Tags == nil {
		rendered.Tags = []string{}
	}
	if t.Quadrant != nil {
		quadrant := gen.Quadrant(*t.Quadrant)
		rendered.Quadrant = &quadrant
	}
	if t.QuadrantAtCompletion != nil {
		quadrant := gen.Quadrant(*t.QuadrantAtCompletion)
		rendered.QuadrantAtCompletion = &quadrant
	}
	if t.CompletedVia != nil {
		via := gen.CompletionSource(*t.CompletedVia)
		rendered.CompletedVia = &via
	}
	return rendered
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// CreateTask adds a task to a quadrant.
func (a *API) CreateTask(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.CreateTaskJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	draft := board.Draft{
		Title:           body.Title,
		Quadrant:        shared.Quadrant(body.Quadrant),
		Description:     stringValue(body.Description),
		DeadlineAt:      body.DeadlineAt,
		DeadlineHasTime: boolValue(body.DeadlineHasTime),
	}
	if body.Color != nil {
		draft.Color = shared.TaskColor(*body.Color)
	}

	created, err := a.board.Create(r.Context(), userID, draft)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, renderTask(created))
}

// CreateSubtask adds a subtask under a parent task.
func (a *API) CreateSubtask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.CreateSubtaskJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	parentID := taskID
	draft := board.Draft{
		ParentID:        &parentID,
		Title:           body.Title,
		Description:     stringValue(body.Description),
		DeadlineAt:      body.DeadlineAt,
		DeadlineHasTime: boolValue(body.DeadlineHasTime),
	}
	if body.Color != nil {
		draft.Color = shared.TaskColor(*body.Color)
	}

	created, err := a.board.Create(r.Context(), userID, draft)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusCreated, renderTask(created))
}

// UpdateTask changes the fields the user may edit.
func (a *API) UpdateTask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.UpdateTaskJSONRequestBody
	present, decoded := decodeFields(w, r, &body)
	if !decoded {
		return
	}

	patch := board.Patch{
		Title:           body.Title,
		Description:     body.Description,
		DeadlineAt:      body.DeadlineAt,
		DeadlineHasTime: body.DeadlineHasTime,
		// Absent and null both decode to a nil pointer, and they mean
		// different things: one leaves the deadline alone, the other clears
		// it. Only the raw body can tell them apart.
		ClearDeadline: present["deadlineAt"] && body.DeadlineAt == nil,
	}
	if body.Color != nil {
		color := shared.TaskColor(*body.Color)
		patch.Color = &color
	}

	updated, err := a.board.Update(r.Context(), userID, taskID, patch)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderTask(updated))
}

// DeleteTask removes a task from the user's views, keeping the row.
func (a *API) DeleteTask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	deleted, err := a.board.Delete(r.Context(), userID, taskID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, deletedResponse{DeletedIDs: deleted})
}

// MoveTask places a task in a quadrant, next to the neighbour it named.
func (a *API) MoveTask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	var body gen.MoveTaskJSONRequestBody
	if !decode(w, r, &body) {
		return
	}

	changed, err := a.board.MoveTask(r.Context(), userID, taskID, board.Move{
		Quadrant: shared.Quadrant(body.TargetQuadrant),
		Before:   body.BeforeTaskId,
		After:    body.AfterTaskId,
	})
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderTaskList(changed))
}

// PromoteTask turns a subtask into a task of its own.
func (a *API) PromoteTask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	// The body is optional: without one the task keeps its parent's quadrant.
	// What arrived decides that, not Content-Length, which a chunked request
	// does not set.
	raw, ok := optionalBody(w, r)
	if !ok {
		return
	}

	var quadrant *shared.Quadrant
	if len(raw) > 0 {
		var body gen.PromoteTaskJSONRequestBody
		if !unmarshalStrict(w, r, raw, &body) {
			return
		}
		if body.Quadrant != nil {
			asked := shared.Quadrant(*body.Quadrant)
			quadrant = &asked
		}
	}

	changed, err := a.board.Promote(r.Context(), userID, taskID, quadrant)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderTaskList(changed))
}

type deletedResponse struct {
	DeletedIDs []uuid.UUID `json:"deletedIds"`
}

func renderTaskList(tasks []task.Task) gen.TaskList {
	rendered := gen.TaskList{Tasks: make([]gen.Task, 0, len(tasks))}
	for i := range tasks {
		rendered.Tasks = append(rendered.Tasks, renderTask(&tasks[i]))
	}
	return rendered
}

func boolValue(v *bool) bool { return v != nil && *v }

// CompleteTask marks a task done and pays out its XP.
func (a *API) CompleteTask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	a.progress(w, r, taskID, a.board.Complete)
}

// ReopenTask returns a task to work and withdraws its XP.
func (a *API) ReopenTask(w http.ResponseWriter, r *http.Request, taskID openapi_types.UUID) {
	a.progress(w, r, taskID, a.board.Reopen)
}

func (a *API) progress(
	w http.ResponseWriter,
	r *http.Request,
	taskID uuid.UUID,
	run func(context.Context, uuid.UUID, uuid.UUID) (*board.Progress, error),
) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	result, err := run(r.Context(), userID, taskID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, renderProgress(result))
}

func renderProgress(result *board.Progress) gen.Progress {
	rendered := gen.Progress{
		Tasks:     make([]gen.Task, 0, len(result.Tasks)),
		XpAwarded: result.XPAwarded,
		// Declared non-nullable: nothing unlocked is an empty list, not null.
		UnlockedAchievements: result.Unlocked,
	}
	for i := range result.Tasks {
		rendered.Tasks = append(rendered.Tasks, renderTask(&result.Tasks[i]))
	}
	if rendered.UnlockedAchievements == nil {
		rendered.UnlockedAchievements = []string{}
	}
	if result.LevelUp != nil {
		rendered.LevelUp = &struct {
			FromLevel int32 `json:"fromLevel"`
			ToLevel   int32 `json:"toLevel"`
		}{FromLevel: result.LevelUp.From, ToLevel: result.LevelUp.To}
	}
	return rendered
}
