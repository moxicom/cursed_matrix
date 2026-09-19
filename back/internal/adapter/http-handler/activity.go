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
