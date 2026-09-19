package cache

import "github.com/google/uuid"

type Scope string

func UserScope(id uuid.UUID) Scope { return Scope("user:" + id.String()) }

type Key struct {
	Scope Scope
	Name  string
	Args  []string
}
