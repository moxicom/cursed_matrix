// Package session holds the vocabulary the session use cases and the stores
// behind them share. It has no behaviour: a port may only declare interfaces,
// and what those interfaces speak in has to live somewhere both sides can see.
package session

// RefreshOutcome is what presenting a refresh token found.
type RefreshOutcome int

const (
	// RefreshUnknown: never issued, expired, revoked, or spent too long ago to
	// be anything but a replay.
	RefreshUnknown RefreshOutcome = iota
	// RefreshValid: the token was live and is now spent.
	RefreshValid
	// RefreshJustRotated: the token was spent moments ago. The usual cause is
	// its own holder asking twice — two tabs waking together, or a response
	// that never arrived — not a thief.
	RefreshJustRotated
)
