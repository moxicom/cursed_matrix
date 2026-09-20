package httphandler

import (
	"net/http"

	gen "github.com/moxicom/cursed_matrix/back/internal/adapter/http-handler/gen"
	"github.com/moxicom/cursed_matrix/back/internal/app/billing"
	"github.com/moxicom/cursed_matrix/back/internal/domain/shared"
	"github.com/moxicom/cursed_matrix/back/internal/domain/user"
)

type plansResponse struct {
	BillingEnabled bool            `json:"billingEnabled"`
	Plans          []gen.PlanOffer `json:"plans"`
}

// ListPlans answers with the pricing page, which needs no account.
func (a *API) ListPlans(w http.ResponseWriter, r *http.Request) {
	offers := a.billing.Offers()

	response := plansResponse{
		BillingEnabled: a.billing.Enabled(),
		Plans:          make([]gen.PlanOffer, 0, len(offers)),
	}
	for i := range offers {
		response.Plans = append(response.Plans, renderOffer(&offers[i]))
	}
	WriteJSON(w, r, http.StatusOK, response)
}

// Checkout buys a plan.
func (a *API) Checkout(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserFrom(r.Context())
	if !ok {
		WriteError(w, r, shared.NewError(shared.CodeSessionExpired, nil))
		return
	}

	granted, err := a.billing.Checkout(r.Context(), userID)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	WriteJSON(w, r, http.StatusOK, a.renderSubscription(granted))
}

func renderOffer(offer *billing.Offer) gen.PlanOffer {
	rendered := gen.PlanOffer{
		Plan:   gen.Plan(offer.Plan),
		Prices: make([]gen.PlanPrice, 0, len(offer.Prices)),
	}
	for _, price := range offer.Prices {
		rendered.Prices = append(rendered.Prices, gen.PlanPrice{
			Language: gen.Language(price.Language),
			Currency: price.Currency,
			Amount:   price.Amount,
		})
	}

	// Unlimited is the absence of a limit, not a number: the client renders no
	// counter for null, and a large integer would draw one.
	if offer.ActiveTaskLimit != user.Unlimited {
		limit := offer.ActiveTaskLimit
		rendered.ActiveTaskLimit = &limit
	}
	if offer.TaskLinkLimit != user.Unlimited {
		limit := offer.TaskLinkLimit
		rendered.TaskLinkLimit = &limit
	}
	return rendered
}

func (a *API) renderSubscription(subscription user.Subscription) gen.Subscription {
	return gen.Subscription{
		Plan:      gen.Plan(subscription.Plan),
		ExpiresAt: subscription.ExpiresAt,
		Expired:   subscription.Expired(a.clock.Now().UTC()),
	}
}
