# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

raidforge computes the **optimal 20-player Mythic World of Warcraft raid composition, per boss**,
from a guild's real roster, Warcraft Logs performance, and a researched raid buff/debuff coverage
matrix — and emits bench lists and recruitment suggestions when the roster is short or missing
coverage. Target game version: **Midnight, Season 1**.

## Status: pre-implementation — read these first

**No application code exists yet.** The repo currently holds only documentation and config files
(copied from a sibling static-site project). Design and conventions are locked; the build has not
started. Before writing anything, read both:

- **`docs/SPEC.md`** — the design ("what & why"): domain model, optimizer objective, connectors, API. Decisions here are locked.
- **`TODO.md`** — the build playbook ("how"): conventions, gotchas, external-API integration notes, and an ordered phased plan (Phase 0–11). This is the authoritative working context.

When those two and this file disagree with the code, the code wins — but right now there is no code,
so SPEC/TODO are the source of truth. Keep them updated as the design evolves.

## Architecture (target — see SPEC.md §6 for the full tree)

Monorepo mirroring sibling geekxflood projects:

```
backend/    Go module github.com/christopherime/raidforge (Go 1.26.x)
  cmd/raidforge/   HTTP server entrypoint (serves API + built frontend, :8080, /healthz)
  internal/        config(CUE) · auth(Battle.net OAuth) · domain · roster · boss · optimizer · connectors · store · server
frontend/   Next.js (TypeScript, App Router) — login, roster switcher, per-boss comp board
data/       versioned datasets: coverage matrix, classes/specs, per-tier boss profiles
chart/      Helm chart (or lives in the external geekxflood/helm-charts repo — see Deployment)
docs/       SPEC.md + ADRs
```

**Data flow** (each external source owns one slice of the problem; all wrapped behind mockable
`connectors` interfaces): Blizzard SSO → *who's in the guild + their talented specs* → Warcraft Logs
v2 GraphQL → *how well each player performs each spec per boss* → Raider.IO → *meta/comp reference* →
static `data/` matrix → *required buff/debuff/utility coverage*. The optimizer combines all of these
**per boss**.

Two facts Blizzard does **not** provide — which specs a player is *willing* to play, and attendance —
come from manual roster edits and optional **wowaudit** enrichment.

### Two architectural keystones (easy to violate, expensive to fix later)

1. **The engine is version-agnostic and data-driven.** All patch-volatile WoW knowledge — raid
   buffs, damage-amp debuffs, Bloodlust/battle-res providers, per-boss profiles and meta rankings —
   lives in versioned files under `data/`, the single source of truth. **Never hardcode these values
   in Go.** The Midnight 12.x seed values are tabulated in SPEC.md §3.3 and TODO.md §3.

2. **The optimizer is the core value.** Per boss, it selects a subset of the roster (= 20) and
   assigns each player an eligible spec, under hard constraints (exact size 20; tank/healer minimums;
   one slot per player; spec ∈ player's eligible set; boss-mandated coverage), maximizing
   `Σ throughput + weighted(buff, debuff, meta, utility coverage) − penalties`. Optimizer weights are
   CUE config, tunable without a rebuild. Two switchable solvers: a **heuristic** (greedy + local
   search) for live UI feedback, and an **exact** ILP/branch-and-bound solver behind a "prove optimal"
   action (tractable because rosters are small, ≤ ~30 players). When no legal/complete comp exists, it
   does gap analysis → quantified recruitment suggestions ("Add a Demon Hunter → +3% raid magic damage").

## Conventions that will bite if ignored

These come from the surrounding geekxflood homelab ecosystem (TODO.md §6); deviating breaks the fit.

- **Go module path is `github.com/christopherime/raidforge`, Go 1.26.x.** Local references for this
  pattern: `../droidfarm` (Go module + version) and `../schedularr`, `../athena` (CUE config).
- **CI uses only `actions/*` GitHub actions plus raw `docker` shell commands** — never `docker/*` or
  other third-party actions (the cluster's self-hosted runners can't reliably fetch them). See
  `.github/workflows/build.yaml`; it already derives the image name from the repo, so no edits are
  needed. **Do not move the repo under the GxF org** — org Actions are suspended, so everything
  publishes from the personal `christopherime` account/registry (`ghcr.io/christopherime/raidforge`).
- **`Dockerfile` and `nginx.conf` are static-site leftovers and must be replaced** (TODO Phase 9).
  The real Dockerfile will be multi-stage (build Next.js → build Go → minimal image), **unprivileged**,
  listening on **`:8080`** with a **`/healthz`** probe. `nginx.conf` is expected to be removed.
- **App config is CUE-backed** (matching `schedularr`/`athena`), not env-only — used for tunable
  optimizer weights.
- **Deployment is GitOps via ArgoCD, and the chart/Application live in *external* repos**, not here:
  Helm chart → `geekxflood/helm-charts`, ArgoCD Application → `geekxflood/applicationset`. Host:
  `raidforge.geekxflood.io`. Unlike the static-site origin, raidforge needs secrets (Blizzard/WCL
  client creds, `SESSION_SECRET`, `DATABASE_URL`) and a database (Postgres recommended).
- **Commit discipline:** branch off `main`; commit/push only when the user asks.

## Building, linting, testing

There is **no build/test tooling yet** (no `go.mod`, no `frontend/package.json`, no Makefile/Taskfile).
TODO Phase 0 calls for a root Makefile or Taskfile mirroring the sibling projects. Until then:

- **YAML lint** is the only configured tooling present: `yamllint .` (config in `.yamllint.yaml`,
  max line length 120). Note its `ignore:` still lists `charts/droidfarm/templates/` — a stale
  copy-paste from another repo; update to the raidforge chart path if a local `chart/` is added.
- Once scaffolded, expect standard Go (`go test ./...`, `golangci-lint run`) and Next.js
  (`lint`, `build`) commands wired through the root build file — add them here when they exist.
- CI build (`.github/workflows/build.yaml`) skips on Markdown-only changes (`paths-ignore: **.md`).

## Local environment notes

- This checkout is at `/Users/christophe/raidforge`. TODO.md refers to `/home/cri/raidforge` and to
  sibling reference projects under `/home/cri/` — paths from another machine.
- Of the referenced siblings, **`cartomancer` (the primary layout reference) and `bench` are absent
  on this machine**; `droidfarm`, `schedularr`, and `athena` are present as `../` siblings and are the
  usable local references for module path, Go version, and CUE patterns.
- TODO.md §1 claims `.claude/settings.json`, `skills-lock.json`, and `.agents/skills/impeccable/`
  already exist — **they do not** in this checkout (`.claude/` contains only `agents/`). Don't rely on
  them being present.
- `CLAUDE*` and `.claude` are listed in `.gitignore`, so this file is intentionally untracked
  (local-only). Adjust `.gitignore` if you want it committed.
- The frontend is meant to use the vendored **`impeccable`** design skill for polished UI (per TODO),
  though it is not vendored in this checkout yet.
