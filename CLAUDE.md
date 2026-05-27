# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

raidforge computes the **optimal 20-player Mythic World of Warcraft raid composition, per boss**,
from a guild's real roster, Warcraft Logs performance, and a researched raid buff/debuff +
capability matrix — and emits bench lists and recruitment suggestions when the roster is short or
missing coverage. Target game version: **Midnight, Season 1**, which ships multiple raids on a staggered release schedule (data model: Season → Raids → Bosses with per-raid release status; SPEC §3.4).

## Status: implementation in progress (toward the v0.5.0 MVP)

Versioning is semver, tagged from `v0.0.1`. **v0.5.0** = a working MVP (domain data + optimizer +
API over a sample roster); **v1.0.0** = first stable release with all components (auth, connectors,
persistence, frontend). The phased plan is in `TODO.md` (Phase 0–11).

The design is the source of truth — read both before changing behaviour:
- **`docs/SPEC.md`** — design ("what & why"): domain model, capabilities, optimizer objective, connectors, API.
- **`TODO.md`** — build playbook ("how"): conventions, gotchas, external-API notes, ordered phases.

What exists so far: the Go backend scaffold (`backend/`, module `github.com/christopherime/raidforge`)
— a minimal HTTP server with `/healthz` — plus build tooling (Makefile, multi-stage Dockerfile) and
the `data/` dataset plan. The optimizer, domain data, connectors, auth, persistence and frontend are
not built yet.

## Architecture (target — see SPEC.md §6 for the full tree)

```txt
backend/    Go module github.com/christopherime/raidforge (Go 1.26.x)
  cmd/raidforge/   HTTP server entrypoint (serves API + built frontend, :8080, /healthz)
  internal/        config(CUE) · auth · domain · roster · boss · optimizer · connectors · store · server
frontend/   JS frontend (framework unresolved — see gotchas)
data/       versioned datasets: capability/coverage matrix, classes/specs, per-tier boss profiles
chart/      Helm chart (or external geekxflood/helm-charts repo)
docs/       SPEC.md + ADRs
```

Data flow: Blizzard SSO → roster + talented specs + **race**; Warcraft Logs → per-player/spec/boss
throughput **and the class/spec makeup of real kills**; Raider.IO → meta reference; static `data/` →
capability/coverage matrix. The optimizer combines all per boss. Willing specs + attendance come
from manual edits / wowaudit.

### Keystones (easy to violate, expensive to undo)

1. **Version-agnostic, data-driven engine.** All patch-volatile WoW knowledge — buffs, debuffs,
   capabilities, providers, boss profiles, meta — lives in versioned `data/`, the single source of
   truth. Capabilities are an **open ID registry**; new spells/bosses/seasons are *data, not code*.
   Never hardcode game facts in Go.

2. **The optimizer is the core.** Per boss, pick 20 of the roster and assign each an eligible spec,
   maximizing throughput + weighted coverage − penalties. Two solvers: heuristic (live) + exact
   ILP/B&B ("prove optimal"). Weights are CUE config.

3. **Capabilities are soft; the logs outrank theory.** Capability/spell coverage is a *weighted
   priority, never a hard constraint* — only structural legality is hard (size 20, role mins, one
   slot/player, spec eligibility). Comps that actually killed a boss in Warcraft Logs get a priority
   boost (`w_empirical`) that can outrank the theoretical "ideal." **Do not reintroduce hard
   capability gates.** (SPEC §3.3, §3.6, §4.)

## Conventions that will bite if ignored

- **Go module `github.com/christopherime/raidforge`, Go 1.26.x.** Local refs: `../droidfarm`
  (module + Go version), `../schedularr` & `../athena` (CUE config, HTTP server, Dockerfile).
- **CI uses only `actions/*` + raw `docker` shell commands** — never `docker/*` or third-party
  actions. Don't move the repo under the GxF org (org Actions suspended → publishes from personal
  `christopherime` → `ghcr.io/christopherime/raidforge`). The Docker build takes
  `--build-arg BUILD_REF=<short-sha>` for the version stamp.
- **App config is CUE-backed** (the real template is `../schedularr/internal/cueconfig`).
- **Deployment is GitOps via ArgoCD**; the Helm chart + Application live in the external
  `geekxflood/helm-charts` and `geekxflood/applicationset` repos. Needs secrets (Blizzard/WCL creds,
  `SESSION_SECRET`, `DATABASE_URL`) + a database.
- **Commit discipline:** semver tags from `v0.0.1`; the user has authorized commit + push for this
  build. Commit footer: `Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>`.

## Building, linting, testing

- `make build` — compile backend to `bin/raidforge` (wraps `go -C backend`).
- `make test` — `go test -race ./...` in backend.
- `make vet` / `make lint` — `go vet` / `golangci-lint run`.
- `make run` — build and run the server (`:8080`, `GET /healthz` → `ok`).
- `yamllint .` — YAML lint (config `.yamllint.yaml`, max line 120; ignores `chart/templates/`).
- CI (`.github/workflows/build.yaml`) builds the Dockerfile on push to `main` and on `v*.*.*` tags
  (skips Markdown-only pushes), pushing `ghcr.io/christopherime/raidforge`.

## Local environment & known gotchas

- This checkout is `/Users/christophe/raidforge`. TODO.md's `/home/cri/...` paths are from another machine.
- Sibling references present locally: `../droidfarm`, `../schedularr`, `../athena`. The docs'
  `cartomancer` and `bench` are **absent** here. athena is the closest HTTP-server + frontend +
  Dockerfile analog; schedularr is the CUE-config + connector-pattern reference.
- Toolchain present: Go 1.26.3, node 26, cue 0.16.1, docker 29.5.2, helm 4.2.0, golangci-lint 2.12.2.
- `CLAUDE.md` is tracked; only `CLAUDE.local.md` and `.claude/` are gitignored.
- **Frontend framework is unresolved:** SPEC says Next.js, but the "single Go binary serves the
  built frontend" packaging goal fits the house Vite+React SPA (athena) better. Decide before
  scaffolding `frontend/`.