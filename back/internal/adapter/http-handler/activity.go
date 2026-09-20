package httphandler

import (
	"net/http"

	openapi_types "github.com/oapi-codegen/runtime/types"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type heatmapResponse struct {
	Days   []gen.ActivityDay `json:"days"`
	Totals heatmapTotals     `json:"totals"`
}

type heatmapTotals struct {
	Created   int `json:"created"`
	Completed int `json:"completed"`
}

// ActivityHeatmap answers with a year of the user's days.
func (a *API) ActivityHeatmap(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	days, err := a.profile.Heatmap(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := heatmapResponse{Days: make([]gen.ActivityDay, 0, len(days))}
	for i := range days {
		day := &days[i]
		response.Totals.Created += day.Created
		response.Totals.Completed += day.Completed
		response.Days = append(response.Days, gen.ActivityDay{
			Date:           openapi_types.Date{Time: day.Date},
			CreatedCount:   day.Created,
			CompletedCount: day.Completed,
			TotalActivity:  day.Total(),
		})
	}
	WriteJSON(w, r, http.StatusOK, response)
}

// GraphOpened notes that the user looked at the graph today.
func (a *API) GraphOpened(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	if err := a.profile.RecordGraphOpened(r.Context(), userID); err != nil {
		WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type eventsResponse struct {
	Items      []gen.ActivityEvent `json:"items"`
	NextCursor *string             `json:"nextCursor"`
}

// ActivityEvents lists what the user did.
func (a *API) ActivityEvents(w http.ResponseWriter, r *http.Request, params gen.ActivityEventsParams) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	limit, cursor := 0, ""
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Cursor != nil {
		cursor = *params.Cursor
	}

	page, err := a.profile.Events(r.Context(), userID, cursor, limit)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := eventsResponse{Items: make([]gen.ActivityEvent, 0, len(page.Entries))}
	for i := range page.Entries {
		entry := &page.Entries[i]
		detail := entry.Metadata
		if detail == nil {
			detail = map[string]any{}
		}
		response.Items = append(response.Items, gen.ActivityEvent{
			Id:         entry.ID,
			Type:       string(entry.Type),
			OccurredAt: entry.OccurredAt,
			LocalDate:  openapi_types.Date{Time: entry.LocalDate},
			TaskId:     entry.TaskID,
			TaskTitle:  entry.TaskTitle,
			Detail:     detail,
		})
	}
	if page.NextCursor != "" {
		next := page.NextCursor
		response.NextCursor = &next
	}
	WriteJSON(w, r, http.StatusOK, response)
}

// ActivityStats answers with the figures above the heatmap.
func (a *API) ActivityStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	stats, err := a.profile.Stats(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	WriteJSON(w, r, http.StatusOK, gen.ActivityStats{
		CreatedLastYear:   stats.CreatedLastYear,
		CompletedLastYear: stats.CompletedLastYear,
		CurrentStreak:     stats.CurrentStreak,
		LongestStreak:     stats.LongestStreak,
		LifetimeXp:        stats.LifetimeXP,
		Level:             stats.Level,
		ActiveTasks:       stats.ActiveTasks,
		ArchivedTasks:     stats.ArchivedTasks,
	})
}
