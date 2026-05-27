# raidforge

> Forge the optimal World of Warcraft raid composition for your guild — **boss by boss** —
> from your real roster, real logs, and the raid-buff math that actually matters.

raidforge logs you in with **Battle.net**, discovers your characters and guild, and builds
your roster automatically. For every boss in the tier it computes the **best 20-player
Mythic composition** your roster can field — balancing raw player performance against raid
buff/debuff coverage and the per-boss meta — and tells you exactly **who to bench** and,
when you're short or missing coverage, **which class to recruit and why**.

> [!NOTE]
> **Status: early design / pre-implementation.** The architecture and domain model are
> locked (see [`docs/SPEC.md`](docs/SPEC.md)); the build is tracked in
> [`TODO.md`](TODO.md). Code is not written yet. This README describes the target system.

---

## Why

Picking a Mythic roster is a constrained optimization problem most guilds solve by gut feel.
The "right" 20 changes **per boss**: a single-target fight rewards different specs than a
council fight, a magic-heavy boss makes a Demon Hunter's **Chaos Brand** worth more than a
Monk's **Mystic Touch**, and you still need to cover Bloodlust, battle rezzes, Stamina,
Intellect, Attack Power, Versatility, dispels, and defensive cooldowns — without benching
your best players. raidforge does that math for you, every pull.

## What it does

- **Per-boss optimal comp** — the best legal, role-balanced 20 for each boss in the tier.
- **Buff/debuff coverage** — guarantees raid-wide buffs, lust, battle rez, and boss-required
  utility are covered; shows a green/red checklist.
- **Performance-aware** — scores each player in each of their specs from their **Warcraft
  Logs** parses, per boss.
- **Bench & swaps** — who sits this fight, and why.
- **Recruitment suggestions** — when the roster can't fill 20 or lacks coverage, it quantifies
  the marginal value of adding an archetype ("Add a Demon Hunter → +3% raid magic damage").
- **Multiple rosters** — pick one or more of your characters; manage each guild as its own
  roster and switch between them.

## How it works

```txt
                Battle.net SSO (wow.profile)
                          │
                          ▼
   ┌─────────────────────────────────────────┐
   │  Blizzard WoW Profile API               │  who's in the guild +
   │  characters → guild → roster → specs    │  what specs they have
   └─────────────────────────────────────────┘
                          │
        roster ───────────┼────────────────────────────────┐
                          │                                │
                          ▼                                ▼
   ┌───────────────────────────┐         ┌───────────────────────────────┐
   │  Warcraft Logs (v2 GQL)   │  how    │  Static data/  (versioned)    │  what's
   │  per player/spec/boss     │  good   │  buff·debuff·lust·brez matrix │  required
   │  parse percentiles        │  ─────► │  + per-boss profiles          │  ─────────┐
   └───────────────────────────┘         └───────────────────────────────┘           │
                          │                                                          │
                          │   Raider.IO ─ meta / comp reference ─────────────────────┤
                          ▼                                                          ▼
                ┌───────────────────────────────────────────────────────────────────────┐
                │  Optimizer (Go)  — per boss                                           │
                │  maximize Σ throughput + buff/debuff/meta/utility − penalties         │
                │  heuristic (live)  ·  exact ILP / branch-and-bound ("prove optimal")  │
                └───────────────────────────────────────────────────────────────────────┘
                          │
                          ▼
            Optimal 20  ·  bench  ·  coverage report  ·  recruitment suggestions
```

The optimizer selects a subset of the roster (= 20) and assigns each player an eligible spec,
subject to hard constraints (raid size, tank/healer minimums, boss-required coverage), while
maximizing a weighted objective of player throughput plus buff/debuff/meta/utility coverage.
Two solvers are available: a **heuristic** for instant feedback as you edit the roster, and an
**exact** solver to prove a composition is optimal.

## Data sources

| Source                    | Provides                                                            | Auth                                        |
| ------------------------- | ------------------------------------------------------------------- | ------------------------------------------- |
| **Blizzard** (Battle.net) | Login, your characters, guild(s), roster, talented specs            | OAuth2 (`wow.profile`) + client credentials |
| **Warcraft Logs** v2      | Per-player/per-boss parse percentiles & spec meta stats             | OAuth2 client-credentials                   |
| **Raider.IO**             | Guild progression + composition/meta reference                      | Public (key optional)                       |
| **wowaudit** *(optional)* | Roster enrichment: willing/alt specs, attendance                    | Per-team API key                            |
| **Static `data/`**        | Raid buff/debuff/lust/brez matrix, classes/specs, per-boss profiles | — (ships in repo)                           |

Blizzard knows *who's in the guild and what specs they have* — but not which specs a player is
*willing* to play. That comes from manual edits or **wowaudit**.

## Raid coverage modeled (Midnight, patch 12.x)

Raid buffs (one each): **Power Word: Fortitude** (Priest, Stamina), **Arcane Intellect** (Mage),
**Battle Shout** (Warrior, AP), **Mark of the Wild** (Druid, Vers), **Skyfury** (Shaman, Mastery).
Damage-amp debuffs: **Chaos Brand** (DH, +magic), **Mystic Touch** (Monk, +physical),
**Hunter's Mark** (Hunter, +all — now raid-wide in Midnight). Plus **Bloodlust** (Shaman/Mage/
Hunter/Evoker), **Battle Res** (Druid/Warlock/DK/Paladin), dispels, and defensive cooldowns.
The matrix is versioned in `data/` and re-verified each patch.

## Stack

- **Backend:** Go 1.26 (`github.com/christopherime/raidforge`) — config via CUE.
- **Frontend:** Next.js (TypeScript, App Router).
- **Persistence:** PostgreSQL (users, sessions, rosters, WCL cache).
- **Packaging:** single multi-stage container (unprivileged `nginx`-free Go binary serving the
  API + built frontend) on `:8080` with a `/healthz` probe.
- **Delivery:** image published to `ghcr.io/christopherime/raidforge` by GitHub Actions;
  deployed to the geekxflood Kubernetes cluster via ArgoCD GitOps.

## Project structure (target)

```txt
raidforge/
├── backend/           Go API + optimizer
│   ├── cmd/raidforge/  server entrypoint
│   └── internal/       config · auth · domain · roster · boss · optimizer · connectors · store · server
├── frontend/          Next.js — login, roster switcher, per-boss comp board
├── data/              versioned datasets: coverage matrix, classes/specs, tier boss profiles
├── chart/             Helm chart (GitOps)
└── docs/              SPEC.md, ADRs
```

## Configuration

The deployment needs these secrets/env vars:

```txt
BLIZZARD_CLIENT_ID        BLIZZARD_CLIENT_SECRET     BLIZZARD_REDIRECT_URL
WCL_CLIENT_ID             WCL_CLIENT_SECRET
SESSION_SECRET            DATABASE_URL
```

The Battle.net OAuth **redirect URL must match the deployed host** (register it at
[develop.battle.net](https://develop.battle.net)). Optimizer weights (buff vs throughput vs
meta) are tunable via CUE config without a rebuild.

## Deployment

Runs on the geekxflood Kubernetes cluster via GitOps (ArgoCD): a push to `main` builds and
pushes `ghcr.io/christopherime/raidforge`, the Helm chart (in `geekxflood/helm-charts`) and
ArgoCD Application (in `geekxflood/applicationset`) deploy it, and `argocd-image-updater`
redeploys on new images. Target host: `raidforge.geekxflood.io` (public via Cloudflare Tunnel).

## Documentation

- [`docs/SPEC.md`](docs/SPEC.md) — full specification: domain model, optimizer, connectors, API.
- [`TODO.md`](TODO.md) — phased build plan and the complete project context.

## Acknowledgments

- Raid buff/debuff data verified against [Icy Veins](https://www.icy-veins.com/wow/) and
  [Warcraft Wiki](https://warcraft.wiki.gg).
- Performance data from [Warcraft Logs](https://www.warcraftlogs.com); meta reference from
  [Raider.IO](https://raider.io); roster from the
  [Blizzard API](https://develop.battle.net); optional enrichment from
  [wowaudit](https://wowaudit.com).

## License

[MIT](LICENSE)
