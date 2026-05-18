# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

benis-phone (Best Enterprise Network Integrated Soft-phone) is a Go-based IVR telephone system for the Anderstorpsfestivalen cultural event. It registers with a SIP PBX (or accepts unauthenticated direct INVITEs in debug mode) and runs an IVR per inbound call.

## Build and Run Commands

```bash
# Build
go build benis-phone.go

# Run against the SIP server configured in the default TOML
./benis-phone

# Custom configuration
./benis-phone -def configurations/atp.toml

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
Required in `creds/creds.json` with keys for S3, Polly, Backend, Trafiklab, Systemet, HTTPServerAuth, SIP, and optionally ElevenLabs.
