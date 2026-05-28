package domain

import "slices"

// Buff is a raid-wide stat buff (one provider class; does not stack). SPEC §3.3.
type Buff struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"` // Class.ID
	Effect   string `json:"effect"`
}

// Debuff is an enemy damage-amp debuff (the magic-vs-physical lever). SPEC §3.3.
type Debuff struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`           // Class.ID
	Kind     string `json:"kind"`               // "magic" | "physical" | "all"
	Effect   string `json:"effect,omitempty"`
}

// Providers lists who grants a capability, by scope (SPEC §3.3). The "player"
// scope is per-player (Player.ManualCapabilities), not listed here.
type Providers struct {
	Specs   []string `json:"specs,omitempty"`   // Spec.ID
	Classes []string `json:"classes,omitempty"` // Class.ID
	Races   []string `json:"races,omitempty"`   // Race.ID
}

// Capability is a named ability category that bosses prioritize (SPEC §3.3).
// IDs form an open registry, e.g. "lust", "battle_rez", "dispel.offensive.magic".
type Capability struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Providers Providers `json:"providers"`
}

// Coverage is the static buff/debuff/capability registry loaded from data/.
type Coverage struct {
	Buffs        []Buff       `json:"buffs"`
	Debuffs      []Debuff     `json:"debuffs"`
	Capabilities []Capability `json:"capabilities"`
}

// ResolveCapabilities returns the set of capability IDs a player provides when
// assigned the given spec: the union of class-, spec-, and race-scoped providers
// from the registry, plus the player's manually declared capabilities. Because
// resolution depends on the assigned spec, a spec choice can grant a capability
// (SPEC §3.3).
func (c Coverage) ResolveCapabilities(p Player, spec string) map[string]bool {
	out := make(map[string]bool)
	for _, cap := range c.Capabilities {
		if slices.Contains(cap.Providers.Classes, p.Class) ||
			slices.Contains(cap.Providers.Specs, spec) ||
			slices.Contains(cap.Providers.Races, p.Race) {
			out[cap.ID] = true
		}
	}
	for _, id := range p.ManualCapabilities {
		out[id] = true
	}
	return out
}
