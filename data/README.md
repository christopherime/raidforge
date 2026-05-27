# data/ — versioned game datasets

raidforge is a **version-agnostic, data-driven engine**: all patch-volatile World of
Warcraft knowledge lives here as versioned data, never hardcoded in Go (see
[`../docs/SPEC.md`](../docs/SPEC.md) §3.7). Adding a new patch or season means adding data
here, not changing code.

Planned layout:

- `classes.{cue,json}` — the 13 classes, their specs, each spec's role, and the
  race → capability table.
- `coverage.{cue,json}` — raid stat buffs, damage-amp debuffs, and the **capabilities**
  registry (capability id → providers, scoped spec/class/race/player). See SPEC §3.3.
- `tiers/<expansion>-s<n>/bosses.{cue,json}` — per-tier boss list and per-boss profiles
  (damage/heal weights, magic-vs-physical split, weighted capability priorities). The
  first tier is `tiers/midnight-s1/` (raid: **Voidspire**). See SPEC §3.4.

Each dataset ships with a CUE schema; the loader validates data against it and fails fast
on malformed input. Capability ids are an **open registry** — new spells or mechanics are
added as data, with no engine changes.
