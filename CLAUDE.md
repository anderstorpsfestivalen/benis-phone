# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

benis-phone (Best Enterprise Network Integrated Soft-phone) is a Go-based IVR telephone system for the Anderstorpsfestivalen cultural event. It registers with a SIP PBX (or accepts unauthenticated direct INVITEs in debug mode) and runs an IVR per inbound call.

## Build and Run Commands

```bash
# Build
go build benis-phone.go

# Default: -source=remote. Subscribes to the worker's ConfigBroker DO
# over WebSocket and hot-swaps the IVR tree when the editor saves.
# In-flight calls keep their old snapshot; new calls pick up the new
# config. Pass -poll to add the legacy HTTP poll fallback (use only when
# the WS upgrade is blocked).
./benis-phone -config simonstorp
./benis-phone -c simonstorp                  # short alias

# Local TOML instead of the worker
./benis-phone -source file -def configurations/atp.toml

# Direct mode: skip PBX registration, accept unauthenticated INVITEs
# (point a softphone at sip:anything@<host>:5060)
./benis-phone -direct -debug

# Disable optional features
./benis-phone -s3=false -http=false
```

## Architecture

### Call Flow
Each inbound SIP call gets its own `Session` (in `core/controller/`) driven by a `SessionManager`. Users navigate menus via DTMF — menus are "Functions" (Fn) defined in the TOML. Key "0" exits the current menu or returns to main.

### Core Components
- **SIP** (`core/sip/`): `Client` registers with the PBX (or listens in direct mode), accepts INVITEs, and constructs per-call `SIPPhone` / `RTPAudioSink` / `RTPAudioSource` / `sipController`. Wire tracing lives in `core/sip/logging.go` (`EnableWireTrace`).
- **Controller** (`core/controller/`): `SessionManager` + per-call `Session` + DTMF `Collector`. Drives the IVR state machine.
- **FlowPhone Interface** (`core/phone/flow.go`): Contract a per-call keypad/hook source presents to the controller — implemented by `core/sip/SIPPhone`.
- **Audio** (`core/audio/`): Shared `AudioSink`/`AudioSource` interfaces, PCM helpers, and `Source` (20ms frame) abstraction. RTP implementations live in `core/sip/`.
- **TTS** (`core/tts/`, `core/polly/`): Pluggable TTS providers (Polly, ElevenLabs) with caching under `haschcache/`.

### Extension System
- **Services** (`extensions/services/`): Plugin-style services (drugslang, traintimes, systemet, etc.) implementing `Service` interface with `Get(input, template, args) string`.
- **Gates** (`extensions/gates/`): Validation/gating logic for conditional menu access.

### Configuration
Menu structure defined in TOML files (`configurations/`). Actions specify destinations (`dst`), services (`srv`), dispatchers, or `livefeed = { device, channel }` to stream a host audio capture device into the call's outbound RTP. Files referenced are in `files/` directory. SIP block lives at `[sip]` in the same TOML; the optional `direct = true` toggle (or `-direct` CLI flag) skips PBX registration. `./benis-phone -list-audio-devices` enumerates capture devices for filling in the livefeed config.

### Credentials
Required in `creds/creds.json` with keys for R2 (S3-compatible Access Key ID + Secret Access Key + AccountID + Bucket — used by `core/filesync/` to mirror the bucket into `files/`), Polly, Backend, Trafiklab, Systemet, HTTPServerAuth, SIP, and optionally ElevenLabs and `PBXConfigToken` (required when `-source=remote`; matches the Worker's `CONFIG_BEARER_TOKEN` secret). The legacy `S3` block is no longer read.

### Web editor (`/ui`)
Single Cloudflare Worker (with bundled static assets via Workers Assets) + D1 + one Durable Object (`ConfigBroker`), served at `ivr.anderstorpsfestivalen.se`. The Worker handles `/api/*` (Cloudflare Access-protected editor CRUD), `/config` (bearer-token TOML / hash pull for backwards-compat polling), `/config/ws` (bearer-token long-lived WebSocket the binary subscribes to for push updates), and falls through to the bundled React build for everything else (Cloudflare Access in front of the hostname, with a bypass policy on `/config*`). Source under `ui/` — React 19 + TS + Tailwind (5-color palette in `tailwind.config.ts`) + pnpm + Vite + Wrangler. `pnpm deploy` builds Vite into `ui/dist` and ships Worker + assets in one shot.

TypeScript types for the IVR config are generated from `core/functions/*.go` by `tools/typegen/`. Run `go generate ./...` from the repo root after editing any struct in `core/functions/`. CI fails if `ui/src/generated/` is out of date.

Local dev: `cd ui && pnpm install`, then `pnpm worker:dev` (Worker on :8787 with local D1) and `pnpm dev` (Vite on :5173 proxying `/api` + `/config` to the Worker). Apply migrations once with `pnpm d1:migrate:local`.

Hot-reload: with `-source=remote`, the binary opens a long-lived WebSocket to `/config/ws?name=...`. The worker's `ConfigBroker` Durable Object (see `ui/worker/durable/configBroker.ts`) holds the subscription and is poked by `PUT /api/configs/:name` after each save. On a `{type:"config-updated"}` event the binary GETs the new TOML via the existing `/config?name=...` endpoint, re-prepares the Definition, and atomically swaps it on the SessionManager — in-flight calls keep their snapshot, new calls get the new config. `SIGUSR1` still forces an immediate reload via the same code path. Pass `-poll` to additionally run the legacy HTTP poll fallback at `-reload-interval` (default 60s).
