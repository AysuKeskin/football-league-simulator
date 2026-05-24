// Package domain holds the core entities, value types, and interfaces
// of the football league simulator.
//
// Nothing in this package depends on a database, HTTP framework, or any
// other infrastructure: services consume the interfaces declared here,
// concrete implementations live elsewhere. This keeps the business model
// portable and trivially testable.
package domain

import "time"

// BaseModel carries the identity and audit timestamps every persisted
// entity shares. It is embedded by Team, League, Match, and any other
// row-backed type so the fields are defined once.
type BaseModel struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Rating bundles the three attributes that drive match simulation.
// Values are in the inclusive range [1, 100]; the constraint is enforced
// at the database boundary (CHECK constraint) and at the API edge.
type Rating struct {
	Attack   int
	Midfield int
	Defense  int
}

// Team is a competitor in one or more leagues.
//
// Team embeds Rating so the simulator can take a Team and read its
// strength attributes directly, and BaseModel for identity/audit.
// Names are unique across the system.
type Team struct {
	BaseModel
	Rating
	Name string
}
