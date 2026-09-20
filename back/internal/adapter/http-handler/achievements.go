package httphandler

import (
	"net/http"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/achievement"
	"github.com/moxicom/cursed_matrix/back/internal/domain/leaderboard"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
)

type achievementsResponse struct {
	Items []gen.Achievement `json:"items"`
}

// ListAchievements answers with the catalogue and how far the user has come.
func (a *API) ListAchievements(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	views, err := a.awards.List(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := achievementsResponse{Items: make([]gen.Achievement, 0, len(views))}
	for i := range views {
		response.Items = append(response.Items, renderAchievement(&views[i]))
	}
	WriteJSON(w, r, http.StatusOK, response)
}

func renderAchievement(view *achievement.View) gen.Achievement {
	return gen.Achievement{
		Code:       view.Code,
		Category:   gen.AchievementCategory(view.Category),
		Threshold:  view.Threshold,
		Progress:   view.Progress,
		RewardXp:   view.RewardXP,
		UnlockedAt: view.UnlockedAt,
	}
}

type leaderboardResponse struct {
	Entries    []gen.LeaderboardEntry  `json:"entries"`
	Me         gen.LeaderboardStanding `json:"me"`
	NextOffset *int                    `json:"nextOffset"`
}

// Leaderboard answers with one page of the public ranking.
func (a *API) Leaderboard(w http.ResponseWriter, r *http.Request, params gen.LeaderboardParams) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	period := leaderboard.PeriodAllTime
	if params.Period != nil {
		period = leaderboard.Period(*params.Period)
	}

	limit, offset := 0, 0
	if params.Limit != nil {
		limit = *params.Limit
	}
	if params.Offset != nil {
		offset = *params.Offset
	}

	ranking, err := a.profile.Leaderboard(r.Context(), userID, period, limit, offset)
	if err != nil {
		WriteError(w, r, err)
		return
	}

	response := leaderboardResponse{
		Entries: make([]gen.LeaderboardEntry, 0, len(ranking.Entries)),
		Me:      gen.LeaderboardStanding{Visible: ranking.Me.Visible, Xp: ranking.Me.XP},
	}
	for i := range ranking.Entries {
		entry := &ranking.Entries[i]
		response.Entries = append(response.Entries, gen.LeaderboardEntry{
			Rank:          entry.Rank,
			UserId:        entry.UserID,
			Username:      entry.Username,
			AvatarUrl:     entry.AvatarURL,
			Xp:            entry.XP,
			Level:         entry.Level,
			CurrentStreak: entry.CurrentStreak,
			IsCurrentUser: entry.UserID == userID,
		})
	}
	if ranking.Me.Visible {
		rank := ranking.Me.Rank
		response.Me.Rank = &rank
	}
	if ranking.NextPage >= 0 {
		next := ranking.NextPage
		response.NextOffset = &next
	}
	WriteJSON(w, r, http.StatusOK, response)
}
