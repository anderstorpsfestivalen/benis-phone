# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

benis-phone (Best Enterprise Network Integrated Soft-phone) is a Go-based IVR telephone system for the Anderstorpsfestivalen cultural event. It registers with a SIP PBX (or accepts unauthenticated direct INVITEs in debug mode) and runs an IVR per inbound call.

## Build and Run Commands

```bash
# Build
go build benis-phone.go

# The only startup mode enrolls/uses a local Ed25519 bridge identity.
./benis-phone -register <registration-id>

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
Runtime credentials are entered per config in the editor and encrypted with `SIP_SECRET_ENCRYPTION_KEY`. SIP passwords retain their per-connection encryption. Approved bridges fetch both through signed `/bridge/runtime` requests and retain them only in memory; the binary never reads `creds.json` or `sip.json`.

### Web editor (`/ui`)
Single Cloudflare Worker (with bundled static assets via Workers Assets) + D1 + one Durable Object (`ConfigBroker`), served at `ivr.anderstorpsfestivalen.se`. The Worker handles Access-protected `/api/*`, public enrollment under `/bridge/enroll*`, signed runtime/hash/WebSocket endpoints under `/bridge/*`, and the React SPA. Cloudflare Access must bypass `/bridge/*` for headless phones.

TypeScript types for the IVR config are generated from `core/functions/*.go` by `tools/typegen/`. Run `go generate ./...` from the repo root after editing any struct in `core/functions/`. CI fails if `ui/src/generated/` is out of date.

Local dev: `cd ui && pnpm install`, then `pnpm worker:dev` (Worker on :8787 with local D1) and `pnpm dev` (Vite on :5173 proxying `/api` + `/bridge` to the Worker). Apply migrations once with `pnpm d1:migrate:local`.

Hot-reload: the binary opens a signed WebSocket to `/bridge/ws`; the validated bridge UUID is the runtime instance ID. On a broker event it fetches `/bridge/runtime`, rebuilds credential-dependent resources, syncs R2, and reconciles SIP. In-flight calls keep their definition snapshot. `SIGUSR1` forces this path; `-poll` adds signed hash polling.
