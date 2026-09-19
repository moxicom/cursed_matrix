package httphandler

import (
	"net/http"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/board"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/task"
)

type tasksResponse struct {
	Tasks     []gen.Task `json:"tasks"`
	Truncated bool       `json:"truncated"`
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
	WriteJSON(w, r, http.StatusOK, tasksResponse{Tasks: rendered, Truncated: result.Truncated})
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
