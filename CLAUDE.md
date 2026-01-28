# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

benis-phone (Best Enterprise Network Integrated Soft-phone) is a Go-based IVR telephone system for the Anderstorpsfestivalen cultural event. It runs on Linux systems and Raspberry Pi with GPIO-connected DTMF phones.

## Build and Run Commands

```bash
# Build
go build benis-phone.go

# Run in virtual mode (keyboard input for testing)
./benis-phone

# Run with physical phone (GPIO/DTMF)
./benis-phone -phone

# Use custom configuration
./benis-phone -def configurations/atp.toml

# Disable optional features
./benis-phone -s3=false -http=false -record=false
```

## Architecture

### State Machine Call Flow
The system uses a call stack for menu navigation. Users press keys (0-9, *, #) to navigate menus defined as "Functions" (Fn). Key "0" exits the current menu or returns to main.

### Core Components
- **Controller** (`core/controller/`): Central hub managing call stack, hook events, DTMF input, and routing actions to handlers
- **FlowPhone Interface**: Strategy pattern allowing swappable input sources - GPIO phone, virtual keyboard, or muxed combination
- **Audio** (`core/audio/`): Playback (MP3, OGG, FLAC, WAV) and recording via beep library
- **Polly** (`core/polly/`): AWS Polly TTS with caching in `haschcache/`

### Extension System
- **Services** (`extensions/services/`): Plugin-style services (drugslang, traintimes, systemet, etc.) implementing `Service` interface with `Get(input, template, args) string`
- **Gates** (`extensions/gates/`): Validation/gating logic for conditional menu access

### Configuration
Menu structure defined in TOML files (`configurations/`). Actions specify destinations (`dst`), services (`srv`), or dispatchers. Files referenced are in `files/` directory.

### Credentials
Required in `creds/creds.json` with keys for S3, Polly, Backend, Trafiklab, Systemet, and HTTPServerAuth.

## System Requirements (Linux/RPI)

```bash
apt install pkg-config libasound2-dev build-essential
```

For Raspberry Pi with USB sound card: install PulseAudio and blacklist onboard `snd_bcm2835`.
