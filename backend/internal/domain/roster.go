// Package domain holds raidforge's core types — classes, specs, roles, races,
// capabilities, players, and the season/raid/boss hierarchy. These types are
// content-agnostic; all patch-volatile game data is loaded from data/ (see
// internal/data and docs/SPEC.md §3.7).
package domain

// Role is the slot a spec fills in a raid.
type Role string

const (
	RoleTank   Role = "tank"
	RoleHealer Role = "healer"
	RoleMelee  Role = "melee"
	RoleRanged Role = "ranged"
)

// Spec is a single specialization of a class, mapped to a role.
type Spec struct {
	ID   string `json:"id"`   // unique, e.g. "mage.frost"
	Name string `json:"name"` // e.g. "Frost"
	Role Role   `json:"role"`
}

// Class is a WoW class and its specs.
type Class struct {
	ID    string `json:"id"`   // e.g. "mage"
	Name  string `json:"name"` // e.g. "Mage"
	Specs []Spec `json:"specs"`
}

// Race is a playable race; some grant race-scoped capabilities (e.g. Blood Elf →
// Arcane Torrent interrupt).
type Race struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Player is a roster member. Blizzard supplies class, race, and talented specs;
// WillingSpecs (the specs they will actually play) and ManualCapabilities come
// from manual edits / wowaudit (SPEC §5).
type Player struct {
	Name               string   `json:"name"`
	Class              string   `json:"class"`                         // Class.ID
	Race               string   `json:"race"`                          // Race.ID
	WillingSpecs       []string `json:"willing_specs"`                 // Spec.ID values
	ManualCapabilities []string `json:"manual_capabilities,omitempty"` // capability IDs (player scope)
}
