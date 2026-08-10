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

# Direct/local mode is an inbound SIP connection in the config. Restrict it
# with allowed_cidrs and point a softphone at its local_port.
./benis-phone -source file -def configurations/atp.toml -debug

# Disable optional features
./benis-phone -s3=false -http=false
```

## Architecture

### Call Flow
Each inbound SIP call gets its own `Session` (in `core/controller/`) driven by a `SessionManager`. Users navigate menus via DTMF — menus are "Functions" (Fn) defined in the TOML. Key "0" exits the current menu or returns to main.

### Core Components
- **SIP** (`core/sip/`): `Supervisor` reconciles multiple isolated `Client` listeners/registrations. Endpoint connections select one graph entrypoint; trunks route exact called numbers plus an optional catch-all. Each call constructs a `SIPPhone` / `RTPAudioSink` / `RTPAudioSource` / `sipController`. Wire tracing lives in `core/sip/logging.go` (`EnableWireTrace`).
- **Controller** (`core/controller/`): `SessionManager` + per-call `Session` + DTMF `Collector`. Drives the IVR state machine.
- **FlowPhone Interface** (`core/phone/flow.go`): Contract a per-call keypad/hook source presents to the controller — implemented by `core/sip/SIPPhone`.
- **Audio** (`core/audio/`): Shared `AudioSink`/`AudioSource` interfaces, PCM helpers, and `Source` (20ms frame) abstraction. RTP implementations live in `core/sip/`.
- **TTS** (`core/tts/`, `core/polly/`): Pluggable TTS providers (Polly, ElevenLabs) with caching under `haschcache/`.

### Extension System
- **Services** (`extensions/services/`): Plugin-style services (drugslang, traintimes, systemet, etc.) implementing `Service` interface with `Get(input, template, args) string`.
- **Gates** (`extensions/gates/`): Validation/gating logic for conditional menu access.

### Configuration
Menu structure is defined in TOML files (`configurations/`). Actions specify destinations (`dst`), services (`srv`), dispatchers, or `livefeed = { device, channel }` to stream a host audio capture device into the call's outbound RTP. Files referenced are in `files/`. `[sip]` contains shared call/recording limits and one or more `[[sip.connection]]` blocks. A connection is `registered` or inbound-only; inbound connections require source CIDRs. `./benis-phone -list-audio-devices` enumerates capture devices for filling in the livefeed config.

### Credentials
Required in `creds/creds.json` with keys for R2 (S3-compatible Access Key ID + Secret Access Key + AccountID + Bucket — used by `core/filesync/` to mirror the bucket into `files/`), Polly, Backend, Trafiklab, Systemet, HTTPServerAuth, and optionally ElevenLabs and `PBXConfigToken` (required when `-source=remote`; matches the Worker's `CONFIG_BEARER_TOKEN` secret). SIP passwords are encrypted per config by the Worker. File-source mode reads a separate connection-id-to-password map from `creds/sip.json` (override with `-sip-secrets`).

### Web editor (`/ui`)
Single Cloudflare Worker (with bundled static assets via Workers Assets) + D1 + one Durable Object (`ConfigBroker`), served at `ivr.anderstorpsfestivalen.se`. The Worker handles `/api/*` (Cloudflare Access-protected editor CRUD), `/config` (bearer-token TOML / hash pull for backwards-compat polling), `/config/ws` (bearer-token long-lived WebSocket the binary subscribes to for push updates), and falls through to the bundled React build for everything else (Cloudflare Access in front of the hostname, with a bypass policy on `/config*`). Source under `ui/` — React 19 + TS + Tailwind (5-color palette in `tailwind.config.ts`) + pnpm + Vite + Wrangler. `pnpm deploy` builds Vite into `ui/dist` and ships Worker + assets in one shot.

TypeScript types for the IVR config are generated from `core/functions/*.go` by `tools/typegen/`. Run `go generate ./...` from the repo root after editing any struct in `core/functions/`. CI fails if `ui/src/generated/` is out of date.

Local dev: `cd ui && pnpm install`, then `pnpm worker:dev` (Worker on :8787 with local D1) and `pnpm dev` (Vite on :5173 proxying `/api` + `/config` to the Worker). Apply migrations once with `pnpm d1:migrate:local`.

Hot-reload: with `-source=remote`, the binary opens a long-lived WebSocket to `/config/ws?name=...&instance_id=...`. The worker's `ConfigBroker` Durable Object holds the subscription and is poked by `PUT /api/configs/:name` after each save. On a config event the binary fetches `/config/runtime`, prepares the Definition, and asks the SIP supervisor to reconcile registrations/listeners/routes. In-flight calls keep their snapshot and entrypoint. The same socket carries per-connection status and heartbeats back to the editor. `SIGUSR1` still forces this path; `-poll` adds the legacy hash poll fallback.
