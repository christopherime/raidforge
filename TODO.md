# raidforge — Build Plan & Context

> **Purpose of this file.** A self-contained build playbook + context primer so any new
> AI/human session can continue building raidforge with the same understanding. Read
> [`docs/SPEC.md`](docs/SPEC.md) for the full design ("what & why"); this file is the
> "how", plus all the conventions and gotchas discovered while setting the project up.
>
> Status: **planning complete, implementation not started.** Only config files (copied
> from the `bench` project) and the spec/this-doc exist so far.

---

## 0. TL;DR — what we're building

A web app that, for a WoW guild, computes the **optimal Mythic raid composition on a
per-boss basis** from the guild's roster. It logs the user in via **Battle.net SSO**,
discovers their characters/guild(s), builds the roster, scores each player/spec/boss
from **Warcraft Logs**, enforces **raid buff/debuff/utility coverage** from a static
dataset, and outputs the best 20-player comp per boss + bench + **recruitment
suggestions** when the roster is short or missing coverage. Targets the **Midnight**
expansion, Season 1.

---

## 1. Project facts & coordinates

| Thing                  | Value                                                                                                                                                                        |
| ---------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| GitHub repo            | `github.com/christopherime/raidforge` (public, MIT, owner `christopherime`)                                                                                                  |
| Local path             | `/home/cri/raidforge`                                                                                                                                                        |
| Container image        | `ghcr.io/christopherime/raidforge` (built by GitHub Actions on push to `main`)                                                                                               |
| Go module path         | `github.com/christopherime/raidforge`                                                                                                                                        |
| Owner / ecosystem      | "geekxflood" (GxF) homelab; deployed via ArgoCD GitOps                                                                                                                       |
| Closest analog project | **`/home/cri/cartomancer`** — monorepo: Go `backend/` (`cmd`/`internal`/`pkg`) + Next.js `frontend/`, plus `chart/`, `docs/`, `.agents/`, `.claude/`. **Mirror its layout.** |
| Other Go refs          | `/home/cri/droidfarm` (uses `github.com/christopherime/...` module path, Go 1.26), `/home/cri/schedularr` & `/home/cri/athena` (CUE config patterns)                         |
| Static-site ref        | `/home/cri/bench` — where the current config files were copied from                                                                                                          |

### Current repo state (what already exists)

- Config copied from `bench`: `.gitignore`, `Dockerfile`, `nginx.conf`, `LICENSE` (MIT,
  "Geekxflood"), `.github/workflows/build.yaml`, `.claude/settings.json`,
  `skills-lock.json`, `.agents/skills/impeccable/` (vendored design skill), `.yamllint.yaml`.
- `docs/SPEC.md` — the full specification (draft v0.3).
- This `TODO.md`.
- ⚠️ **`Dockerfile` + `nginx.conf` are for a STATIC site and must be replaced** (see Phase 9).

---

## 2. Locked decisions

| Area                       | Decision                                                                                                                                     |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Authentication             | **Blizzard SSO** (Battle.net OAuth2, `wow.profile` scope)                                                                                    |
| Roster source              | **Blizzard WoW Profile API** (character → guild → guild roster); wowaudit optional enrichment                                                |
| Performance / best-spec    | **Warcraft Logs API** v2 (GraphQL, OAuth2 client-credentials)                                                                                |
| Composition/meta reference | **Raider.IO API**                                                                                                                            |
| Buff/debuff data           | **Static, researched** dataset in `data/` (Midnight seed below)                                                                              |
| Frontend                   | **Next.js** (monorepo `frontend/`)                                                                                                           |
| Game target                | **Midnight Season 1**, engine is **version-agnostic / data-driven**                                                                          |
| Solver                     | **Both, switchable** — heuristic (live UI) + exact ILP/B&B ("prove optimal")                                                                 |
| Backend                    | Go (1.26.x), module `github.com/christopherime/raidforge`                                                                                    |
| Config                     | CUE-backed (tunable optimizer weights)                                                                                                       |
| Persistence                | **Required** (multi-user + multi-roster + OAuth tokens). Recommend **Postgres**; SQLite acceptable for single-instance v1. *(open: confirm)* |

---

## 3. Domain knowledge (WoW) — encode this in `data/`

> All of this is **patch-volatile** and lives in versioned datasets under `data/`, NOT
> hardcoded in Go. The values below are the **Midnight (patch 12.x)** seed, researched
> and verified May 2026. Re-verify each major patch.

### 3.1 Classes / specs / roles

13 classes, each spec maps to a role: **Tank / Healer / Melee DPS / Ranged DPS**.
Classes: Warrior, Paladin, Hunter, Rogue, Priest, Death Knight, Shaman, Mage, Warlock,
Monk, Druid, Demon Hunter, Evoker. A *player* declares the set of specs they're **willing
to play** — alt-spec flexibility is the optimizer's main lever.

### 3.2 Composition rules

- **Mythic = fixed 20 players.** (Heroic/Normal flex 10–30 is out of scope for v1.)
- Hard role minimums (boss-dependent): typically **2 tanks**, **4–6 healers**, rest DPS.

### 3.3 Raid stat buffs (one provider class each; do not stack)

| Buff                  | Provider | Effect (Midnight)               |
| --------------------- | -------- | ------------------------------- |
| Power Word: Fortitude | Priest   | +5% Stamina                     |
| Arcane Intellect      | Mage     | +3% Intellect                   |
| Battle Shout          | Warrior  | +5% Attack Power                |
| Mark of the Wild      | Druid    | +3% Versatility                 |
| Skyfury               | Shaman   | +2% Mastery (+ empowered autos) |

### 3.4 Enemy damage-amp debuffs (the magic-vs-physical lever for boss profiles)

| Debuff        | Provider     | Effect (Midnight)                                                            |
| ------------- | ------------ | ---------------------------------------------------------------------------- |
| Chaos Brand   | Demon Hunter | +3% magic damage taken                                                       |
| Mystic Touch  | Monk         | +5% physical damage taken                                                    |
| Hunter's Mark | Hunter       | +3% all damage taken — **Midnight change**: now raid-wide, consistent uptime |

### 3.5 Capabilities (the unified utility model)

Every boss-relevant ability is a **capability**: an ID + provider rules in `data/` that a boss can
**prioritize**. One open, data-driven registry replaces the old separate lists (lust, brez,
dispels, interrupts, soaks, immunities, raid defensives, movement). **Soft, never hard blockers**
(§4). Provider scope: **spec** / **class** / **race** (from Blizzard, e.g. Arcane Torrent →
interrupt) / **player** (manual: professions, one-offs). A player's caps = union over those for
their assigned spec. Dispels carry a **direction**: *friendly* (off an ally, by school) vs
*offensive* (strip a buff/enrage **off an enemy** — purge/spellsteal/soothe).

Midnight seed (illustrative; `data/` authoritative, re-verify each patch):

- `lust` — Shaman, Mage (Time Warp), Hunter pet (Primal Rage), Evoker.
- `battle_rez` — Druid, Warlock, Death Knight, Paladin.
- `dispel.friendly.{magic,curse,poison,disease}` — by class/spec.
- `dispel.offensive.magic` — Mage (Spellsteal), Priest (Dispel Magic), Shaman (Purge). *(e.g. a Voidspire boss values purging a buff off the boss.)*
- `dispel.offensive.enrage` — Druid (Soothe), Hunter (Tranq Shot).
- `interrupt` — most melee/casters; Blood Elf (Arcane Torrent).
- `raid_defensive` — Priest Barrier, DH Darkness, Shaman Spirit Link, Warrior Rallying Cry, Paladin Aura Mastery, DK AMZ.
- `immunity` / `soak` / `external_cd` / `movement` — boss-mechanic specific.

> Only **scarce** capabilities sway the comp; ones every player has (generic soaks, potions) don't.

### 3.6 Boss profile (per boss, in `data/`)

Each boss carries weights/flags that reshape the optimizer objective:

- Damage-pattern weights: single-target vs cleave vs sustained-AoE.
- Magic-vs-physical raid-damage split → sets relative value of Chaos Brand vs Mystic Touch.
- Healing intensity → drives healer count (4/5/6).
- **Capability priorities** (§3.5): capabilities this boss values, each weighted + optional count (e.g. `dispel.offensive.magic` high, `≥1`). **Soft, not gates.** Plus lust timing, expected brez.
- Per-boss meta-spec rankings (a spec can be S-tier on one boss, C-tier on another).

> **Research gap:** the **Voidspire** (Midnight S1) boss list and per-boss profiles — including each
> boss's weighted capability priorities — still need sourcing/curation. The buff/debuff + capability
> *provider* matrix is verified; boss *requirements* are TBD content.

### 3.7 Empirical prior & future-proofing

- **Proven beats ideal.** Per boss, pull the class/spec makeup of real WCL kills and reward comps
  that resemble them (`w_empirical`); this can outweigh theoretical capability coverage. Capability
  coverage is **soft, never a hard blocker** — a comp that killed the boss in the logs must stay
  selectable.
- **Future content = data, not code.** Capabilities are an open ID registry in `data/`; tiers live
  under `data/tiers/<expansion>-s<n>/`. Adding Midnight S2 (new bosses, spells, capabilities) is a
  data change. Go types stay generic (`Capability`, `BossProfile`), so no hard recode per season.

---

## 4. Architecture

### 4.1 Monorepo layout (target)

```txt
raidforge/
├── backend/                  module github.com/christopherime/raidforge
│   ├── cmd/raidforge/         HTTP server entrypoint
│   ├── internal/
│   │   ├── config/            CUE-backed config (optimizer weights, tunables)
│   │   ├── auth/              Battle.net OAuth (login, token refresh, sessions)
│   │   ├── domain/            classes, specs, buffs, roles (loads data/)
│   │   ├── roster/            roster model, multi-roster selection, eligible-spec sets
│   │   ├── boss/              tier + per-boss profile loading
│   │   ├── optimizer/         heuristic + exact solvers, objective, gap analysis
│   │   ├── connectors/        blizzard / wowaudit / warcraftlogs / raiderio clients
│   │   ├── store/             persistence (users, sessions, rosters, WCL cache)
│   │   └── server/            HTTP handlers, routing, middleware
│   └── pkg/                   exported helpers (if any)
├── frontend/                 Next.js app (login, roster switcher, boss board)
├── data/                     versioned datasets: coverage matrix, classes/specs,
│                             tier boss lists + boss profiles, meta rankings
├── chart/                    Helm chart (for geekxflood GitOps) — or lives in geekxflood/helm-charts
├── docs/                     SPEC.md, ADRs
├── Dockerfile  .github/  .agents/  .claude/  LICENSE  TODO.md
```

### 4.2 The optimizer (core value)

**Problem:** select a subset of roster players (= 20) and assign each an eligible spec,
**per boss**.

- **Hard constraints (structural only):** exact size 20; tank/healer minimums; one player ≤ one
  slot; spec ∈ player's eligible set. Capability coverage is **soft/weighted, never a hard gate**.
- **Objective (maximize):**

  ```txt
  score = Σ player_throughput(spec, boss)
        + w_buff      · buff_coverage_completeness
        + w_debuff    · (chaos_brand?·magicWeight + mystic_touch?·physWeight)
        + w_meta      · meta_alignment(specs, boss)
        + w_caps      · Σ capability_weight(boss) · satisfied?      // soft — never a gate
        + w_empirical · similarity(comp, proven WCL kill comps)     // can be set to dominate
        − penalties(role imbalance, benching a top performer)
  ```

  Weights live in CUE config.
- **Solvers:** heuristic (greedy seed + local search, instant, live UI) and exact
  (ILP / branch-and-bound, "prove optimal"). Rosters are small (≤ ~30 players, ≤ ~3 specs
  each) so exact is tractable.
- **Gap analysis:** when no legal/complete comp exists, report **marginal value of adding
  archetype X** ("Add a Demon Hunter → +3% raid magic damage"; "Add a Curse dispeller →
  fight requires it"). This is the recruitment-suggestion feature.

### 4.3 Auth + multi-roster

- Login via Battle.net OAuth → discover all the user's characters → each character's guild
  is a candidate roster.
- User **designates one or more characters** → manages **multiple rosters** (one per guild),
  switchable in the UI.
- Blizzard gives *who's in the guild* + *talented specs*, **not** willing/alt specs or
  attendance → fill those via manual edit and/or wowaudit.

---

## 5. External API integration notes & gotchas

### 5.1 Blizzard (Battle.net) — auth + roster

- Register an API client at **develop.battle.net** → `BLIZZARD_CLIENT_ID` / `BLIZZARD_CLIENT_SECRET`.
  Must register an **OAuth redirect URL matching the deployed host** (and a localhost one for dev).
- **OAuth2 Authorization Code**, hosts `oauth.battle.net` (`/authorize`, `/token`), scope
  **`wow.profile`**.
- **User token** + `wow.profile` → **Account Profile Summary** `GET /profile/user/wow`
  (namespace `profile-{region}`) → lists the account's characters.
- **Client-credential token** → Game Data / Profile endpoints:
  - Character Profile `GET /profile/wow/character/{realmSlug}/{charName}` → guild, active spec.
  - Character Specializations `GET /profile/wow/character/{realmSlug}/{charName}/specializations`.
  - Guild Roster `GET /data/wow/guild/{realmSlug}/{guildSlug}/roster` (namespace `profile-{region}`).
- **Region-aware:** `us`/`eu`/`kr`/`tw` hosts + `profile-{region}` namespaces; CN is separate.
- Guild roster gives members but NOT their specs → fetch per-character specializations
  (many calls → **cache hard**).

### 5.2 Warcraft Logs (WCL) v2 — performance / best spec

- **GraphQL** API at `https://www.warcraftlogs.com/api/v2/client`. **OAuth2 client-credentials**
  (`WCL_CLIENT_ID` / `WCL_CLIENT_SECRET`, registered in WCL settings).
- **Point-based rate limits** → batch GraphQL queries, cache results in `store/`.
- Use for: per-character/per-boss parse percentiles, and zone/spec meta statistics.

### 5.3 Raider.IO — meta/comp reference

- Public REST at `https://raider.io/api/v1`, API key optional. Docs at `raider.io/api`
  (JS-rendered — confirm exact paths/fields at impl time).
- Guild profile with `fields=raid_progression,raid_rankings,members`; character profile for
  class/spec/gear. Use to inform per-boss meta rankings + sanity-check comps.

### 5.4 wowaudit — optional roster enrichment

- Per-team API key (Bearer). Docs at `wowaudit.com` (JS-rendered — confirm at impl).
- Adds willing/alt specs, attendance, roles the guild already tracks. Optional; user supplies
  their own team key.

---

## 6. Conventions & constraints — **MUST follow**

These are derived from the existing geekxflood projects; deviating breaks the homelab fit.

1. **Go module path** = `github.com/christopherime/raidforge`. Go **1.26.x**.
2. **GitHub Actions: use ONLY `actions/*` actions + plain `docker` shell commands.** Do NOT
   introduce `docker/*` or other third-party actions — the cluster's self-hosted runners
   can't reliably download them. (This is why `.github/workflows/build.yaml` builds via raw
   `docker build/login/push`. Keep it that way.) The workflow already auto-derives the image
   name from `$REPO` → `ghcr.io/christopherime/raidforge`, no edit needed.
3. **Org Actions are suspended** for the GxF org — that's why everything publishes from the
   **personal `christopherime`** account/registry. Don't move the repo under the org.
4. **Dockerfile**: multi-stage, **unprivileged**, listens on **`:8080`**, exposes
   **`/healthz`** for k8s probes. Replace the copied static-site Dockerfile (Phase 9).
5. **LICENSE**: MIT, "Geekxflood" — keep.
6. **CUE** for app config (optimizer weights/tunables), matching `schedularr`/`athena`.
7. **Frontend** uses the vendored **`impeccable`** design skill (`.agents/skills/impeccable`,
   enabled in `.claude/settings.json`). Use it for polished UI.
8. **Deployment = GitOps via ArgoCD** in the **geekxflood** repos:
   - Helm chart → `geekxflood/helm-charts` (`charts/raidforge`).
   - ArgoCD Application → `geekxflood/applicationset` (App-of-Apps pattern).
   - `argocd-image-updater` redeploys on new images.
   - Host: `raidforge.geekxflood.io` (public via Cloudflare Tunnel) + `*.local.geekxflood.io`
     (internal via Cilium Gateway) — confirm.
   - Unlike `bench` (static), raidforge needs **secrets** (Blizzard/WCL client creds, session
     key, DB URL) and a **database** → the Helm chart is more involved than bench's.
9. **Commit discipline:** branch off `main`; commit/push only when the user asks. Commit
   message footer: `Co-Authored-By: Claude <noreply@anthropic.com>`.

### Secrets / env the deployment needs

`BLIZZARD_CLIENT_ID`, `BLIZZARD_CLIENT_SECRET`, `BLIZZARD_REDIRECT_URL`,
`WCL_CLIENT_ID`, `WCL_CLIENT_SECRET`, `SESSION_SECRET`, `DATABASE_URL`,
(per-user, stored not env: wowaudit team key, Raider.IO optional).

---

## 7. Build plan (phased, ordered)

> Recommended order optimizes for an early end-to-end vertical slice. Each phase should be a
> branch + PR. Write tests alongside (Go `testing`, table-driven; mock connectors).

### Phase 0 — Scaffolding

- [ ] `backend/go.mod` → `module github.com/christopherime/raidforge`, Go 1.26.x.
- [ ] `backend/cmd/raidforge/main.go` — minimal HTTP server, `/healthz` returning `ok`.
- [ ] `frontend/` — `create-next-app` (TypeScript, App Router).
- [ ] Root `Makefile` or `Taskfile` (build backend, build frontend, run, lint, test) — match
      whatever `cartomancer` uses.
- [ ] `data/` dir with a README describing dataset schemas.
- [ ] Update root `README.md` (replace any leftover bench content).

### Phase 1 — Backend foundation

- [ ] `internal/config` — CUE schema + loader for optimizer weights & server config.
- [ ] `internal/server` — router (chi or stdlib `net/http` mux), middleware (logging, CORS,
      session), `/healthz`, graceful shutdown.
- [ ] Structured logging + config-driven port (`:8080`).

### Phase 2 — Domain model + static data

- [ ] `internal/domain` — generic, content-agnostic types: Class, Spec, Role, Race, Buff, Debuff,
      **Capability** (open ID registry; provider scope spec/class/race/player; dispel direction
      friendly/offensive; linked spells), Player. Capability *resolution* (union over a player's
      class/spec/race/manual) lives here.
- [ ] `data/classes.{cue,json}` — 13 classes, specs, roles, **race→capability** table.
- [ ] `data/coverage.{cue,json}` — §3.3–3.5 incl. the **capabilities → providers** registry (Midnight seed).
- [ ] `data/tiers/midnight-s1/bosses.{cue,json}` — Voidspire boss list + per-boss profiles (§3.6),
      each with **weighted capability priorities** (soft). **Needs content research/curation**
      (boss list, magic/phys weights, healer counts).
- [ ] Loader + validation (fail fast on malformed data).

### Phase 3 — Authentication (Blizzard SSO)

- [ ] `internal/auth` — Battle.net OAuth2 (authorize redirect, callback, token exchange,
      refresh), session creation, CSRF/state handling.
- [ ] `internal/store` (minimum for auth) — users, sessions, encrypted OAuth tokens.
- [ ] Endpoints `/api/auth/login`, `/api/auth/callback`, `/api/auth/logout`,
      `/api/me/characters`.

### Phase 4 — Connectors

- [ ] Define `connectors` interfaces (so each is mockable).
- [ ] **Blizzard** connector — Account Profile (characters), Character Profile (guild/spec),
      Guild Roster, Specializations. Region-aware. Caching.
- [ ] **Warcraft Logs** connector — client-credential token, GraphQL queries for parses, meta
      stats, **and kill-comp data** (class/spec makeup of successful kills → empirical prior §3.6).
      Cache in `store`.
- [ ] **Raider.IO** connector — guild/character profiles for meta reference.
- [ ] **wowaudit** connector (optional) — team-key roster enrichment.

### Phase 5 — Roster + persistence

- [ ] `internal/roster` — build roster from Blizzard; multi-roster selection per user.
- [ ] `store` — persist users' designated rosters, members, manual willing-spec edits.
- [ ] Endpoints `/api/rosters` (CRUD), `/api/rosters/{id}`, `/api/connect/wowaudit`.

### Phase 6 — Optimizer

- [ ] `internal/optimizer` — objective function (§4.2): structural hard constraints + soft weighted
      capability coverage + empirical kill-comp prior. Role constraints.
- [ ] Heuristic solver (greedy + local search) first.
- [ ] Exact solver (ILP via a Go LP lib, or custom branch-and-bound) behind a flag.
- [ ] Gap analysis / recruitment suggestions.
- [ ] Heavy unit tests with crafted rosters + expected comps.

### Phase 7 — API layer

- [ ] `/api/tiers/{tier}/bosses`, `/api/optimize/{boss}`, `/api/optimize` (batch),
      `/api/wcl/parses`, `/api/raiderio/guild`.
- [ ] DTOs + JSON contracts the frontend consumes. Document in `docs/`.

### Phase 8 — Frontend (Next.js, use impeccable skill)

- [ ] "Sign in with Battle.net" + character discovery screen.
- [ ] Roster switcher (multi-guild) + roster editor (tag willing specs, refresh WCL).
- [ ] Boss board: tabs per boss → optimal 20, bench, coverage checklist (green/red),
      recruitment suggestions. Solver toggle (heuristic vs prove-optimal).
- [ ] Advanced weights panel (exposes CUE weights).

### Phase 9 — Containerization + CI

- [ ] **Replace** `Dockerfile` with multi-stage: build Next.js → build Go → minimal final
      image serving API + built frontend (Go `embed` FS or co-serve). Unprivileged, `:8080`,
      `/healthz`.
- [ ] **Remove `nginx.conf`** (static-site leftover) unless used as a reverse proxy sidecar.
- [ ] Confirm `.github/workflows/build.yaml` builds the new Dockerfile (it should, as-is).
- [ ] Add a CI lint/test job (Go test + `golangci-lint`, frontend lint/build) — `actions/*` only.

### Phase 10 — Deployment (GitOps)

- [ ] Helm chart in `geekxflood/helm-charts/charts/raidforge`: Deployment, Service,
      Ingress/Gateway, **Secret** wiring (Blizzard/WCL/session), DB (Postgres dependency or
      external), probes on `/healthz`.
- [ ] ArgoCD Application in `geekxflood/applicationset`.
- [ ] Register the production OAuth redirect URL with Blizzard.
- [ ] Verify `argocd-image-updater` picks up new images.

### Phase 11 — Polish

- [ ] ADRs in `docs/` for major decisions (solver choice, datastore, embed vs co-serve).
- [ ] Curate/verify Midnight S1 boss profiles with real raid data once available.
- [ ] Observability (metrics/logs), error handling for API rate limits & token expiry.

---

## 8. Open questions still needing the owner's decision

1. **Credentials on hand?** Blizzard client (ID/secret + redirect URL) — *required for SSO*;
   WCL client; optional wowaudit team key. Gates live wiring vs mocks.
2. **Datastore:** Postgres (recommended for k8s/multi-user) vs SQLite (simplest v1)?
3. **Healer-count policy:** optimizer chooses within a boss-defined range, or fixed per boss?
4. **Region scope:** single region (EU/US) or multi-region? Affects Blizzard namespaces +
   which WCL/Raider.IO realms we query.
5. **Deploy host** confirm `raidforge.geekxflood.io` + geekxflood GitOps wiring.
6. **Midnight S1 boss list + profiles** — needs content sourcing/curation (research gap).
7. **wowaudit / Raider.IO exact endpoint shapes** — confirm against live (JS-rendered) docs.

---

## 9. References

- This repo's spec: [`docs/SPEC.md`](docs/SPEC.md)
- Blizzard API: <https://develop.battle.net/documentation> · OAuth guide:
  <https://develop.battle.net/documentation/guides/using-oauth>
- Warcraft Logs API v2: <https://www.warcraftlogs.com/api/docs>
- Raider.IO API: <https://raider.io/api>
- wowaudit: <https://wowaudit.com/>
- Midnight raid buffs (verified source): <https://www.icy-veins.com/wow/news/players-choosing-a-midnight-main-need-to-check-their-raid-buff/>
- Bloodlust / battle-res providers: <https://warcraft.wiki.gg/wiki/Bloodlust_effect> ·
  <https://dotesports.com/wow/news/all-classes-with-a-battle-resurrection-ability-in-world-of-warcraft>
- Layout reference project: `/home/cri/cartomancer`
