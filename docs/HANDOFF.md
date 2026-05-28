# Handoff snapshot

Single entry point for any new development session (human or AI) picking up
raidforge. Captures **where we are**, **what's next**, the **research data
already gathered**, and the **design rules that took multiple rounds to
settle** — so the next session doesn't have to rediscover them.

> **Read order:** this file → `docs/SPEC.md` (design) → `TODO.md` (phased plan)
> → `CLAUDE.md` (conventions snapshot). Everything else is in the code.

---

## Tags shipped

| Tag | What | Status |
|---|---|---|
| `v0.0.1` | Buildable base — Go module, `GET /healthz`, Makefile, multi-stage Dockerfile (replaces the original static-site one) | shipped, CI image green |
| `v0.1.0` | CUE-backed config + server foundation — stdlib mux, request-logging & panic-recovery middleware, slog logger, graceful shutdown, tunable optimizer weights | shipped, CI image green |

The container image at `ghcr.io/christopherime/raidforge` is published by GitHub
Actions on each push to `main` and on `v*.*.*` tags. The Dockerfile takes a
`--build-arg BUILD_REF=<short-sha>` for the version stamp; that's all the
workflow passes — keep it that way.

---

## In flight — `v0.2.0` (domain model + static data)

### Round 1 — committed (`feat(domain): add coverage and roster types for raid composition`, commit `93a4d5a`)

- `backend/internal/domain/roster.go` — `Role`, `Spec`, `Class`, `Race`, `Player`.
- `backend/internal/domain/coverage.go` — `Buff`, `Debuff`, `Providers`, `Capability`, `Coverage` + `Coverage.ResolveCapabilities(player, spec) map[string]bool` (union over class/spec/race-scoped providers, plus the player's manual capabilities).

Compiles + vets clean (`go -C backend vet ./internal/domain`).

### Round 2 — what to build to ship `v0.2.0`

#### 2a. Finish the domain types

- `backend/internal/domain/season.go`:
  - `ReleaseStatus` (`"released"` | `"upcoming"`), `Release` (`Status`, optional `Date`).
  - `Season` (`ID`, `Expansion`, `Name`, `Raids []Raid`).
  - `Raid` (`ID`, `Name`, `Location`, `Release`, `Bosses []Boss`) + `(Raid) Released() bool`.
  - `(Season) ReleasedRaids() []Raid`.
  - `Boss` (`ID`, `Name`, `Order`, `Profile BossProfile`).
  - `BossProfile` (`MagicDamage`, `PhysicalDamage` floats, `Healers int`, `CapabilityPriorities []CapabilityPriority`, optional `Note`).
  - `CapabilityPriority` (`Capability string`, `Weight float64`, optional `MinCount int`, optional `Note`).
- Tests:
  - `coverage_test.go` — ResolveCapabilities returns expected ID set for a sample `Coverage` + `Player` (cover class-, spec-, race-, manual-scope cases; assert a non-provider class does NOT get the capability).
  - `season_test.go` — `ReleasedRaids` filters `upcoming` out; `Raid.Released` reports correctly.

#### 2b. Data files (under repo `data/`)

- `data/classes.json` — 13 classes + their specs (id `class.spec`, e.g. `mage.frost`; name; role) + the playable races list.
  - Classes: Warrior (Arms/Fury melee, Protection tank), Paladin (Holy healer, Protection tank, Retribution melee), Hunter (Beast Mastery/Marksmanship ranged, Survival melee), Rogue (Assassination/Outlaw/Subtlety melee), Priest (Discipline/Holy healer, Shadow ranged), Death Knight (Blood tank, Frost/Unholy melee), Shaman (Elemental ranged, Enhancement melee, Restoration healer), Mage (Arcane/Fire/Frost ranged), Warlock (Affliction/Demonology/Destruction ranged), Monk (Brewmaster tank, Mistweaver healer, Windwalker melee), Druid (Balance ranged, Feral melee, Guardian tank, Restoration healer), Demon Hunter (Havoc melee, Vengeance tank), Evoker (Devastation/Augmentation ranged, Preservation healer). 39 specs total.
- `data/coverage.json` — buffs (5: PW:Fortitude/Priest stamina, Arcane Intellect/Mage int, Battle Shout/Warrior AP, Mark of the Wild/Druid versatility, Skyfury/Shaman mastery), debuffs (3: Chaos Brand/DH magic, Mystic Touch/Monk physical, Hunter's Mark/Hunter all), and the **capabilities registry** with these IDs (open registry — add freely): `lust`, `battle_rez`, `dispel.friendly.magic`, `dispel.friendly.curse`, `dispel.friendly.poison`, `dispel.friendly.disease`, `dispel.offensive.magic` (Mage Spellsteal / Priest Dispel Magic / Shaman Purge), `dispel.offensive.enrage` (Druid Soothe / Hunter Tranq Shot), `interrupt` (most melee/casters + race `bloodelf` via Arcane Torrent), `raid_defensive` (Priest Barrier, DH Darkness, Shaman Spirit Link, Warrior Rallying Cry, Paladin Aura Mastery, DK AMZ), `immunity`, `soak`, `external_cd`, `movement`.
- `data/seasons/midnight-s1/season.json` — season metadata + raids array (no bosses inline):
  ```
  {id, expansion, name, raids: [{id, name, location, release: {status, date}, order}]}
  ```
- `data/seasons/midnight-s1/raids/<raid-id>/bosses.json` — one file per released raid. JSON array of `Boss` objects with full profiles.

##### Boss content (researched, real — sources at bottom)

**The Voidspire** (Voidstorm, released 2026-03-18, Mythic 2026-03-24). 6 bosses in pull order:
1. Imperator Averzian
2. Vorasius
3. Fallen-King Salhadaar
4. Vaelgor and Ezzorak
5. **Lightblinded Vanguard** — magic/aura mechanics → boss profile carries a **high `dispel.offensive.magic` priority** (this *is* the offensive-dispel example the owner flagged early in the design).
6. Crown of the Cosmos (Xal'atath)

**The Dreamrift** (Harandar, released 2026-03-17, Mythic 2026-03-24). Intentionally single-boss raid by design:
1. Chimaerus, the Undreamt God

**March on Quel'Danas** (Isle of Quel'Danas, released 2026-03-31). 2 bosses:
1. Belo'ren, Child of Al'ar — Void/Light alternating phases; magic-heavy.
2. Midnight Falls (L'ura) — endboss.

**4th raid** — the owner explicitly said a 4th is "coming soon." Sources don't name it yet. Model as a placeholder upcoming raid (e.g. `tba-s1-r4`, `release.status = "upcoming"`, no bosses file) to exercise the release-status model — **do not omit it**.

Boss profiles are seed data (approximate magic/physical splits, healer counts, capability priorities). Mark them clearly as seed; they're meant to be refined.

#### 2c. Loader (`backend/internal/data`)

- Embedded CUE schemas under `backend/internal/data/schema/`: `classes.cue`, `coverage.cue`, `season.cue` (`#Season` for `season.json`), `bosses.cue` (`#Bosses: [...#Boss]` for the array file).
- `backend/internal/data/load.go` — `Load(dir, seasonID string) (*Dataset, error)`. **Mirror the pattern in `backend/internal/config/config.go`**: `cuecontext.New()` → `CompileString(schemaCUE)` → `LookupPath(cue.ParsePath("#X"))` → `ctx.Encode(jsonMap)` → `Unify` → `Validate(cue.Concrete(true))` → `Decode(&out)`. For the bosses array, encode the `[]any` from json.Unmarshal and unify with `#Bosses`.
- `Dataset` = `{Classes []domain.Class, Races []domain.Race, Coverage domain.Coverage, Season domain.Season}`.
- Load order: `classes.json` → `coverage.json` → `seasons/<id>/season.json` → for each raid whose bosses file exists (presumably every `released` raid), read `raids/<id>/bosses.json` into `Raid.Bosses`. Upcoming raids stay empty.
- Tests in `load_test.go` against the repo's `data/`: assert 13 classes, 39 specs, season has 4 raids (3 released + 1 upcoming), `ReleasedRaids()` returns 3, Voidspire has 6 bosses with Lightblinded Vanguard at order 5, and a capability resolution sanity (e.g. a Blood Elf mage on `mage.frost` gets `lust`, `dispel.offensive.magic`, `interrupt`).

#### 2d. Config rename — `data.tier` → `data.season`

With Season the canonical concept, rename the config field for clarity (pre-1.0, no migration story needed):
- `backend/internal/config/schema.cue` — `data: { dir: string | *"data", season: string | *"midnight-s1" }`.
- `backend/internal/config/config.go` — `rawConfig.Data.Season` (json tag `season`), public `Data.Season`.
- `backend/internal/config/config_test.go` — assert `cfg.Data.Season == "midnight-s1"`.
- `backend/configs/raidforge.example.yaml` — `data: { dir: data, season: "midnight-s1" }`.

#### 2e. Verify & ship

```sh
make vet && make build && make test
docker build --build-arg BUILD_REF=v0.2.0 -t raidforge:v0.2.0-local .
docker run -d --rm -p 8090:8080 --name rf raidforge:v0.2.0-local
curl -fsS http://127.0.0.1:8090/healthz
docker stop rf
```

Then commit (suggested: `feat(domain,data): season → raids → bosses + capability registry (v0.2.0)`), `git tag -a v0.2.0 -m "..."`, push `main` and the tag. CI builds the image.

> **Dockerfile gotcha for later (not v0.2.0):** the server doesn't load `data/` yet (still just `/healthz`), so the current Dockerfile (which copies only `backend/`) is fine through v0.2.0. When the server starts loading data (v0.4.0), the Dockerfile must `COPY data/ /app/data` (or wherever `config.Data.Dir` points) and the server should default to that path.

---

## Roadmap (locked)

| Tag | Milestone |
|---|---|
| `v0.0.1` ✅ | scaffold |
| `v0.1.0` ✅ | config + server foundation |
| `v0.2.0` 🔄 | domain + static data (Round 1 done; Round 2 = §2a–2e above) |
| `v0.3.0` | optimizer core — objective per SPEC §4, structural hard constraints, heuristic solver (greedy + local search), gap analysis (recruitment suggestions). Heavy unit tests with crafted rosters. |
| `v0.4.0` | roster from a sample JSON file + API (`GET /api/seasons/{id}/raids`, `GET /api/raids/{id}/bosses`, `POST /api/optimize/{boss}`). Wire optimizer to API. **Dockerfile starts copying `data/`.** |
| `v0.5.0` | **MVP** — end-to-end: load sample roster + data → per-boss optimization → comp + bench + coverage report + recruitment suggestions, all via API. |
| v0.5 → v1.0 | Blizzard SSO (Battle.net OAuth), live WCL / Raider.IO / wowaudit connectors, persistence (Postgres or SQLite), frontend, Helm chart + ArgoCD wiring. |

---

## Design rules (do NOT violate)

1. **Capabilities are SOFT, never hard constraints.** Only *structural legality* is hard: exact raid size 20, role minimums (tanks/healers), one slot per player, spec ∈ player's eligible set. Capability/spell coverage is a **weighted priority** (`w_caps`). Comps that actually killed the boss in Warcraft Logs get a priority boost (`w_empirical`) that can outrank theoretical "ideal." **Do not reintroduce hard capability gates.** (SPEC §3.3, §3.6, §4.) — This was an explicit owner correction.
2. **All patch-volatile WoW data is in `data/` as content, never in Go.** Capabilities form an **open ID registry**; new spells / bosses / seasons are added as data, not code. Never hardcode game facts (class lists, dispel providers, boss names) in Go — the types (`Capability`, `Boss`, `Raid`, `Season`) are content-agnostic.
3. **Season → Raids → Bosses + per-raid release status.** A season ships **multiple raids on a staggered schedule** (Midnight S1 = 3 + 1 upcoming). Never assume one-raid-per-season; always honor `release.status` (`released` | `upcoming`). The optimizer runs over the bosses of *released* raids the user has selected.
4. **Go module is `github.com/christopherime/raidforge`, Go 1.26.x.** Original-machine toolchain: Go 1.26.3, node 26, cue 0.16.1, docker 29.5.2, helm 4.2.0, golangci-lint 2.12.2.
5. **CI uses only `actions/*` + raw `docker` shell commands.** Self-hosted runners can't reliably download third-party actions; never introduce `docker/*` or other third-party actions. The org's Actions are suspended → publishes from personal `christopherime` → `ghcr.io/christopherime/raidforge`. Don't move the repo under the GxF org. Don't change what build args the workflow passes (`BUILD_REF`).
6. **CUE-backed config.** The schema (`backend/internal/config/schema.cue`) is the source of truth; YAML config is unified against it (defaults filled, unknown fields rejected, validated), then decoded into typed Go. **Mirror this exact pattern for `internal/data`** — one schema + helper per file, decode into `domain.*` types.
7. **Commit discipline.** Semver tags from `v0.0.1`. The owner has authorized commit + push for this build. Commit-footer convention: `Co-Authored-By: <your AI identity> <noreply@anthropic.com>` (or your provider's equivalent). Tag minor bumps at each milestone; patch bumps for in-between sub-steps when it makes sense. Md-only pushes skip CI (workflow has `paths-ignore: ["**.md"]`); any push containing non-md files triggers the Docker build.

---

## Open architectural decisions (still to resolve)

- **Frontend framework.** SPEC §8 specifies **Next.js (App Router)**, but the README and SPEC §9 require a **single Go binary serving the built frontend** (nginx-free, single container). Next.js App Router needs a Node runtime or `next export` (which discards server features) — both conflict with the packaging goal. The only available local sibling with a frontend (`../athena` on the original machine) uses **Vite + React + TS SPA** (react-router, zustand, axios, lucide-react, reactflow, zod) served by Go via `http.FileServer` with SPA fallback — fits the packaging goal cleanly. The cited Next.js precedent (`cartomancer`) is absent. **Decide before scaffolding `frontend/` (v0.5 → v1.0).**
- **Persistence.** Postgres (SPEC lean, multi-user + k8s + WCL cache) vs SQLite (sibling `../schedularr` uses `mattn/go-sqlite3` + `internal/store/migrations`). Not needed before v0.5.0 (MVP runs on a sample roster file); decide for v0.5 → v1.0.

---

## Local reference projects (only present on the original machine)

The SPEC/TODO cite `cartomancer` and `bench` as references — both **absent** on the original machine. The usable local references there are:
- `../athena` — closest **HTTP-server + JS frontend + Dockerfile** analog. `cmd/athena/main.go` is the composition-root pattern (already mirrored in raidforge's `cmd/raidforge/main.go`). Its multi-stage Dockerfile (frontend-builder → backend-builder with upx → alpine non-root) is the template raidforge's Dockerfile is adapted from.
- `../schedularr` — the real **CUE-backed-config** template (`internal/cueconfig` uses `cuelang.org/go`). Also the per-source connector pattern (`internal/external/{sonarr,radarr,jellyfin,tunarr}` ≈ raidforge's planned `internal/connectors/{blizzard,wcl,raiderio,wowaudit}`); `internal/store` + `migrations/`.
- `../droidfarm` — current `github.com/christopherime/...` + Go 1.26 + CI conventions.

**On a different machine, these aren't present.** Fall back to the patterns already in this repo: `backend/internal/config/` mirrors schedularr's CUE loader; `backend/internal/server/` is an athena-style minimal server. The patterns are self-contained in the codebase now.

---

## Research data — sources

Midnight Season 1 raids (used to seed `data/seasons/midnight-s1/`):
- [Icy Veins — Midnight S1 raid guide](https://www.icy-veins.com/wow/midnight-season-1-raid-guide)
- [Blizzard — Midnight Raid Overview & Schedule](https://news.blizzard.com/en-us/article/24264416/midnight-raid-overview-and-schedule)
- [Wowhead — The Voidspire](https://www.wowhead.com/guide/midnight/raids/the-voidspire-overview-location-rewards-bosses)
- [Wowhead — Dreamrift / Chimaerus](https://www.wowhead.com/guide/midnight/raids/the-dreamrift-chimaerus-boss-strategy-abilities-rewards)
- [Wowhead — March on Quel'Danas](https://www.wowhead.com/guide/midnight/raids/march-on-quel-danas-overview-location-rewards-bosses)
