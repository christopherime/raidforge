# raidforge — Specification

> Status: **draft v0.3** · Target: **Midnight Season 1** (Mythic) · Last updated by design discussion.

## 1. Problem statement

Given a guild's **roster** (players and the specs each can play, with Warcraft Logs
performance data) and a **target raid tier**, raidforge produces the **optimal raid
composition on a per-boss basis**. "Optimal" means a legal, role-balanced group that
maximizes player throughput *and* satisfies the buff / debuff / utility coverage and
meta-spec preferences that matter for that specific boss.

When the roster cannot fully staff the raid (fewer than 20 for Mythic) or is missing
critical coverage, raidforge emits **recruitment suggestions**: which class/spec
archetype to add, and the quantified reason.

This is a **constrained combinatorial optimization** problem, recomputed per boss
because each boss weights the objective differently.

## 2. Decisions locked

| Area                       | Decision                                                                          |
| -------------------------- | --------------------------------------------------------------------------------- |
| Authentication             | **Blizzard SSO** (Battle.net OAuth2, `wow.profile` scope) — log in, discover characters |
| Roster source              | **Blizzard WoW Profile API** (character → guild → Guild Roster); wowaudit optional enrichment |
| Performance / best-spec    | **Warcraft Logs API** (v2 GraphQL, OAuth2 client-credentials)                     |
| Composition/meta reference | **Raider.IO API** (guild progression + comp reference)                            |
| Buff/debuff data           | **Static, researched** dataset in `data/` (Midnight seed in §3.3)                 |
| Capabilities model         | Unified, data-driven capabilities (lust/brez/dispels/interrupts/soaks/immunities/…); providers scoped player/race/class/spec; **open registry** — new spells/caps for future seasons are data, not code |
| Constraint hardness        | Only **structural legality** is hard (size 20, role mins, slot, spec eligibility). Capability coverage is **soft/weighted, never a blocker**; comps proven in WCL kill logs are prioritized over the theoretical "ideal" |
| Frontend                   | **Next.js** (monorepo `frontend/`, mirroring cartomancer)                         |
| Game target                | **Midnight Season 1**, via a **version-agnostic, data-driven engine**             |
| Solver                     | **Both, switchable** — heuristic for live UI, exact (ILP/B&B) for "prove optimal" |
| Backend                    | Go, module `github.com/christopherime/raidforge`                                  |
| Config                     | CUE-backed (matching geekxflood projects) for tunable weights                     |
| Deploy                     | Container → `ghcr.io/christopherime/raidforge`, GitOps via ArgoCD (geekxflood)    |

## 3. Domain model

### 3.1 Classes, specs, roles

13 classes × their specs, each mapped to a role: **Tank / Healer / Melee DPS / Ranged
DPS**. A *player* declares the set of specs they can play; alt-spec flexibility is a
primary lever the optimizer pulls.

### 3.2 Composition rules

- **Mythic = fixed 20.** (Heroic/Normal flexible 10–30 is out of scope for v1.)
- Hard role minimums, boss-dependent: typically **2 tanks**, **4–6 healers**, rest DPS.

### 3.3 Coverage matrix (versioned data, not hardcoded — researched for Midnight 12.x)

The engine tracks coverage **categories**, each provided by certain classes. This table
ships in `data/` per patch because it shifts between patches. Values below are sourced
for **Midnight (patch 12.x)** and must be re-verified each major patch.

**Raid stat buffs** (one provider class each; do not stack):

| Buff                  | Provider    | Effect (Midnight)                      |
| --------------------- | ----------- | -------------------------------------- |
| Power Word: Fortitude | **Priest**  | +5% Stamina                            |
| Arcane Intellect      | **Mage**    | +3% Intellect                          |
| Battle Shout          | **Warrior** | +5% Attack Power                       |
| Mark of the Wild      | **Druid**   | +3% Versatility                        |
| Skyfury               | **Shaman**  | +2% Mastery (+ empowered auto-attacks) |

**Enemy damage-amp debuffs** (the magic-vs-physical lever for boss profiles):

| Debuff        | Provider         | Effect (Midnight)                                                        |
| ------------- | ---------------- | ------------------------------------------------------------------------ |
| Chaos Brand   | **Demon Hunter** | +3% magic damage taken                                                   |
| Mystic Touch  | **Monk**         | +5% physical damage taken                                                |
| Hunter's Mark | **Hunter**       | +3% all damage taken (Midnight change: now consistent uptime, raid-wide) |

**Capabilities** (the unified utility model — presence/count, *not* throughput). Every
boss-relevant ability is a **capability**: a named entry that some providers grant and that a
boss can prioritize. This replaces the old separate lists — lust, battle res, dispels,
interrupts, soaks, immunities, raid defensives, movement are all capabilities — with one
**open, data-driven registry**: a capability is just an ID + provider rules in `data/`, so a new
spell or mechanic in a future season is *added as data, never code* (see §3.7).

*Capabilities are soft.* They are **strong weighted priorities, never hard blockers** — a comp is
never rejected for missing one (§4). Only structural legality (§3.2) is hard.

*Provider scope* — a capability is granted at one of four scopes; a player's capability set for a
given spec assignment is the union across all four:

- **spec** — only some specs of a class (talent/spec-gated).
- **class** — any spec of the class.
- **race** — derived from the character's race (e.g. Arcane Torrent → interrupt). Race comes from Blizzard.
- **player** — manually declared (professions, one-offs Blizzard doesn't expose).

*Dispels carry a direction axis*: **friendly** (cleanse a debuff off an ally, by school
Magic/Curse/Poison/Disease) vs **offensive** (strip a beneficial/enrage effect *off an enemy* —
purge / spellsteal / soothe). Distinct capabilities; a boss can prioritize either.

Midnight seed (illustrative; `data/` is authoritative, re-verified each patch):

| Capability (id)                                | Providers (Midnight, illustrative)                                                |
| ---------------------------------------------- | --------------------------------------------------------------------------------- |
| `lust`                                         | Shaman (Bloodlust/Heroism), Mage (Time Warp), Hunter pet (Primal Rage), Evoker    |
| `battle_rez`                                   | Druid, Warlock, Death Knight, Paladin                                             |
| `dispel.friendly.{magic,curse,poison,disease}` | by class/spec                                                                     |
| `dispel.offensive.magic`                       | Mage (Spellsteal), Priest (Dispel Magic), Shaman (Purge)                          |
| `dispel.offensive.enrage`                      | Druid (Soothe), Hunter (Tranq Shot)                                              |
| `interrupt`                                    | most melee/caster specs; Blood Elf (Arcane Torrent)                              |
| `raid_defensive`                               | Priest Barrier, DH Darkness, Shaman Spirit Link, Warrior Rallying Cry, Pally Aura Mastery, DK AMZ |
| `immunity` / `soak` / `external_cd` / `movement` | varies                                                                          |

> Provider lists seeded for Midnight S1 from Icy Veins / Warcraft Wiki; `data/` is authoritative.
> A capability only influences selection when it is **scarce** — one every player has doesn't.

### 3.4 Seasons, raids, and boss profiles

A **season** (content tier, e.g. Midnight S1) ships **multiple raids**, and raids **release on a
staggered schedule** — some are live while others are still upcoming, and in later seasons/patches
they arrive on different patches. The model is therefore **Season → Raids → Bosses**: a season
lists its raids; each raid has a **release status** (`released` | `upcoming`, with an optional
patch/date) and its own ordered bosses. The optimizer runs per boss, over the bosses of the
*released* raids the user selects.

Each boss carries weights/flags that reshape the objective:

- Damage-pattern weights: single-target vs cleave vs sustained-AoE.
- Magic-vs-physical raid-damage split → sets the relative value of Chaos Brand vs Mystic Touch.
- Healing intensity → drives healer count (4 / 5 / 6).
- **Capability priorities** (§3.3): the capabilities this boss values, each with a **weight** and optional target count (e.g. `dispel.offensive.magic` weighted high, `≥1`). Soft preferences, not gates — missing one lowers score, never disqualifies (§4).
- Per-boss meta-spec rankings (a spec can be S-tier on a council fight and C-tier on a patchwerk).

### 3.5 Player performance

Per player × spec × boss: a throughput score from Warcraft Logs parse percentiles
(or sim-normalized DPS/HPS). Falls back to spec meta-average when a player has no logs
on that boss.

### 3.6 Empirical composition prior (proven kills outrank theory)

Theory says what *should* work; the logs say what *did*. Per boss, raidforge pulls from Warcraft
Logs the **class/spec makeup of real successful kills** (not just individual parses) and rewards
candidate comps that resemble these proven, boss-killing compositions. A comp that has
demonstrably downed the boss is preferred *even if it lacks an "ideal" capability* — the kill log
is ground truth. This is a weighted objective term (§4, `w_empirical`) that can be tuned to
**outweigh** theoretical capability coverage. Raider.IO comp data is a secondary signal of the
same kind.

### 3.7 Versioning & future content (future-proof by construction)

The domain is patch-volatile, so all of it is versioned **data**, never Go code:

- **Open capability registry** — a capability is just an ID + provider rules in `data/`. A new
  spell, ability, or mechanic in Midnight S2 (or any later tier) is added as *data*; the engine
  enumerates no specific capability, class, or boss in code.
- **Per-season, multi-raid datasets** — `data/seasons/<expansion>-s<n>/` holds the season's
  **raids**, each under `raids/<raid>/` with its bosses + a release status (`released` | `upcoming`);
  raids can go live on different patches. The coverage/capability matrix is versioned per patch.
  A new raid, season, or patch = new data files, no recode.
- **Stable generic types** — `Season{raids}`, `Raid{release, bosses}`, `Boss{profile}`,
  `Capability{id, providers, spells}`, `BossProfile{weights, capabilityPriorities[]}` and friends
  are content-agnostic.
- The active tier/patch dataset is selected by config.

## 4. Optimization engine

**Decision variables:** select a subset of roster players (= raid size) and assign each
an eligible spec.

**Hard constraints (structural legality only):** exact raid size (20); tank/healer minimums; one
player ≤ one slot; spec ∈ player's eligible set. **Capability coverage is *not* hard** — it is a
weighted objective term, so a comp is never rejected for missing a capability (a comp that killed
the boss in the logs must stay selectable).

**Objective (maximize), per boss:**

```txt
score = Σ player_throughput(spec, boss)
      + w_buff      · buff_coverage_completeness
      + w_debuff    · (chaos_brand?·magicWeight + mystic_touch?·physWeight)
      + w_meta      · meta_alignment(chosen specs, boss)
      + w_caps      · Σ capability_weight(boss) · satisfied?         // soft — never a gate
      + w_empirical · similarity(comp, proven WCL kill comps)        // §3.6 — can be set to dominate
      − penalties(role imbalance, benching a top performer)
```

Weights — including `w_caps` and `w_empirical` — live in CUE config, tunable without code changes.
Setting `w_empirical` high makes compositions proven in the logs outrank theoretical capability
coverage, so raidforge never insists on a textbook comp the data contradicts.

**Gap analysis / recruitment suggestions:** when the roster can't field a legal 20 (structural)
or the best comp leaves a high-value capability gap, report the **marginal value of adding
archetype X** ("Add a Demon Hunter → unlocks Chaos Brand, ~+4% raid magic damage"; "Add a Curse
dispeller → this boss values it heavily"). Framed as score gains, never hard blockers.

**Solvers (switchable):**

- **Heuristic** — greedy seed + local search; instant, for live UI feedback as the user edits the roster.
- **Exact** — ILP / branch-and-bound; provably optimal, behind a "prove optimal" action. Rosters are small (≤ ~30 players, ≤ ~3 specs each), so this is tractable.

**Output per boss:** chosen 20 + bench list + ranked recruitment suggestions + coverage report.

## 5. Authentication & roster discovery

### 5.1 Blizzard SSO login flow

raidforge authenticates users via **Battle.net OAuth2** (Authorization Code grant, scope
`wow.profile`). On login we:

1. Call the **Account Profile Summary** (`/profile/user/wow`, user token + `wow.profile`)
   to enumerate **all the user's WoW characters** (name, realm, class, race, level). *Race* feeds
   race-scoped capabilities (§3.3).
2. For each character of interest, resolve its **guild** via the Character Profile, then
   pull the **Guild Roster** (`/data/wow/guild/{realm}/{guild}/roster`, client-credential
   token) to get the full member list.
3. Per member, fetch **Character Specializations** to know which specs they have.

This makes login the entry point that *discovers characters → infers guild → builds the
roster* with zero manual entry.

### 5.2 Multiple characters → multiple rosters

A user can **designate one or more of their characters**, each typically in a different
guild, to manage **multiple distinct rosters**. The app stores the user's selected
characters; each selected character's guild is a separately optimizable roster, and the
user switches between them. (This resolves the earlier multi-guild question: yes — accounts
with multiple rosters.)

> Blizzard data gives *who is in the guild* and *what specs a character has talented* — but
> **not** which specs a player is *willing* to play, nor attendance. That "eligible spec set"
> is filled by **manual edits** and optionally **wowaudit** enrichment.

## 5a. Data sourcing — multi-connector architecture

Each external source is wrapped behind a Go `connectors` interface (mockable, cacheable,
rate-limit-aware). Sources, each owning a distinct slice of the problem:

| Connector                       | Owns                                                                                                                     | Auth                                                              | Notes                                                                                       |
| ------------------------------- | ------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| **Blizzard** (SSO + WoW Profile)| **Identity + roster** — the user's characters, their guild(s), guild member list, per-character talented specs + race           | OAuth2: user token (`wow.profile`) + client-credential token      | Primary roster source; see §5.1–5.2. Region-aware (`profile-{region}` namespace)            |
| **wowaudit** (optional)         | **Roster enrichment** — willing/alt specs, attendance, roles the guild already tracks                                    | Per-team API key (Bearer)                                         | Augments Blizzard data with info Blizzard doesn't expose; user connects their team key      |
| **Raider.IO**                   | **Composition/meta reference** — guild raid progression and the comps top guilds run                                     | Public API, key optional                                          | Informs per-boss meta rankings and "what comps work" sanity checks                          |
| **Warcraft Logs** (v2 GraphQL)  | **Best specs & performance** — per-player/per-boss parse percentiles, spec meta statistics, plus the class/spec makeup of real kills (empirical comp prior, §3.6)                               | OAuth2 client-credentials (`WCL_CLIENT_ID` / `WCL_CLIENT_SECRET`) | Point-based rate limits → cache aggressively, batch queries. The primary "best spec" signal |
| **Static `data/`** (researched) | **Coverage matrix** — raid buffs/debuffs, lust, brez, utility (see §3.3); class/spec list; boss list + profiles per tier | none (ships in repo)                                              | Single source of truth for buff/debuff; versioned per patch                                 |

**Flow:** Blizzard SSO defines *who's in the roster and what specs they have* → wowaudit/manual
add *what they're willing to play* → Warcraft Logs scores *how well each plays each spec per
boss* → Raider.IO + static meta rankings inform *what's good on this boss* → the static coverage
matrix scores *buff/debuff/utility coverage* (soft), and Warcraft Logs' real kill comps bias toward
what *actually works* (§3.6). The optimizer (§4) combines all per boss.

**Credential model:** Blizzard needs a registered API client (`BLIZZARD_CLIENT_ID` /
`BLIZZARD_CLIENT_SECRET`, deployment secrets) — used for both the OAuth login redirect and
client-credential Game/Profile calls. WCL needs its own registered credentials. wowaudit
uses the guild's own team key, supplied by the user at connect-time. Raider.IO is keyless.
Exact wowaudit/Raider.IO endpoint paths to be confirmed against live docs at implementation.

## 6. Monorepo structure (mirrors cartomancer)

```txt
raidforge/
├── backend/                  module github.com/christopherime/raidforge
│   ├── cmd/raidforge/         HTTP server entrypoint
│   ├── internal/
│   │   ├── config/            CUE-backed config (weights, tunables)
│   │   ├── auth/              Battle.net OAuth (login, token refresh, sessions)
│   │   ├── domain/            classes, specs, buffs, roles
│   │   ├── roster/            roster ingest, player/spec model, multi-roster selection
│   │   ├── boss/              tier + per-boss profiles
│   │   ├── optimizer/         heuristic + exact solvers
│   │   ├── connectors/        Blizzard / wowaudit / Warcraft Logs / Raider.IO
│   │   ├── store/             persistence (users, sessions, selected rosters, WCL cache)
│   │   └── server/            HTTP handlers, API
│   └── pkg/                   exported helpers (if any)
├── frontend/                 Next.js — roster input, per-boss comp board, suggestions
├── data/                     versioned datasets (coverage matrix, tiers, boss profiles)
├── chart/                    Helm chart (geekxflood GitOps)
├── docs/                     ADRs + this SPEC
├── Dockerfile  .github/  .agents/  .claude/  LICENSE
```

## 7. HTTP API (sketch)

| Method | Path                       | Purpose                                                                                                       |
| ------ | -------------------------- | ------------------------------------------------------------------------------------------------------------- |
| `GET`  | `/api/auth/login`          | start Battle.net OAuth redirect (`wow.profile`)                                                               |
| `GET`  | `/api/auth/callback`       | OAuth callback → create session, discover characters                                                          |
| `POST` | `/api/auth/logout`         | end session                                                                                                   |
| `GET`  | `/api/me/characters`       | the logged-in user's WoW characters (from Blizzard)                                                           |
| `POST` | `/api/rosters`             | designate a character/guild as a managed roster                                                               |
| `GET`  | `/api/rosters`             | list the user's managed rosters (multi-guild)                                                                 |
| `GET`  | `/api/rosters/{id}`        | one roster (members + eligible specs); refreshes from Blizzard guild roster                                  |
| `POST` | `/api/rosters/{id}`        | manual edit (willing specs, add/remove) + optional wowaudit enrich                                            |
| `POST` | `/api/connect/wowaudit`    | attach the guild's wowaudit team key to enrich a roster                                                       |
| `GET`  | `/api/tiers/{tier}/bosses` | list bosses + profiles for a tier                                                                             |
| `POST` | `/api/optimize/{boss}`     | one boss (body: roster id, raid size, solver=heuristic\|exact) → comp + bench + suggestions + coverage        |
| `POST` | `/api/optimize`            | all bosses in the tier for a roster (batch)                                                                   |
| `GET`  | `/api/wcl/parses?...`      | proxy/cache Warcraft Logs lookups (best-spec scores)                                                          |
| `GET`  | `/api/raiderio/guild?...`  | proxy/cache Raider.IO guild progression/comp reference                                                        |
| `GET`  | `/healthz`                 | k8s probe                                                                                                     |

## 8. Frontend (Next.js)

- **Login**: "Sign in with Battle.net" → character discovery screen.
- **Roster switcher**: pick which character/guild roster to manage (multi-guild).
- **Roster editor**: review discovered members, tag willing/eligible specs, pull/refresh WCL parses.
- **Boss board**: tabs per boss; shows the optimal 20, bench, coverage checklist
  (green/red per category), and recruitment suggestions.
- **Solver toggle**: heuristic (live) vs "prove optimal" (exact).
- **Weights panel** (advanced): expose the CUE weights for power users.

## 9. Deployment notes

- The `Dockerfile` + `nginx.conf` copied from `bench` are for a *static* site and will
  be **replaced** by a multi-stage build: build Next.js → build Go → final image serves
  the API and the built frontend (embedded FS or co-served). `nginx.conf` likely removed.
- GitHub Actions workflow is reused as-is (it builds whatever `Dockerfile` exists →
  `ghcr.io/christopherime/raidforge`).
- GitOps: Helm chart + ArgoCD Application in the geekxflood repos; target host
  e.g. `raidforge.geekxflood.io` (to confirm). WCL secrets injected via the cluster.

## 10. Open questions

1. **Credentials on hand** — (a) **Blizzard API client** (ID/secret, with an OAuth redirect
   URL registered at develop.battle.net) — required for SSO; (b) Warcraft Logs API client;
   (c) optional wowaudit team key. Determines what we wire live vs. mock.
2. **Persistence is now required** (multi-user + multi-roster + OAuth tokens). Pick a store:
   Postgres (matches your stack?) vs. SQLite for v1. Also caches WCL parses (rate limits).
3. ~~Multi-guild / auth~~ — **resolved**: Blizzard SSO accounts, multiple rosters (§5.2).
4. **Healer-count policy** — let the optimizer choose healer count within a boss-defined
   range, or fix it per boss?
5. **Deploy host** — confirm `raidforge.geekxflood.io` and the geekxflood GitOps wiring.
   Note: the Battle.net OAuth redirect URL must match the deployed host.
6. **Region scope** — single region (EU/US) or multi-region? Affects Blizzard namespace
   handling and which WCL/Raider.IO realms we query.
7. **wowaudit/Raider.IO endpoint shapes** — confirm exact paths/fields against live docs
   when implementing connectors (couldn't fully scrape; both are JS-rendered docs).
