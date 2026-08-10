![benis-phone Logo](/logo.jpeg)

# benis-phone
Best Enterprise Network Integrated Soft-phone, aka benis-phone!

# Requirements
To run on Linux (tested with Debian / Ubuntu / Raspbian) the following packets are required: 

* pkg-config 
* libasound2-dev 
* build-essential 

Install with: apt install pkg-config libasound2-dev build-essential 

# Sound on RPI with an USB-card
If running on a RPI, install pulseaudio and disable the onboard soundcard by commenting out the following in /lib/modprobe.d/aliases.conf

```
#options snd-usb-audio index=-2
```

Also add a blacklist entry in /etc/modprobe.d/raspi-blacklist.conf

```
blacklist snd_bcm2835
```

Reboot the RPI!

# Credentials and bridge enrollment

Runtime credentials are configured in the web editor for each config. The
editor API returns only configured/unconfigured indicators. Credential bundles
and per-connection SIP passwords are encrypted in D1 with the
`SIP_SECRET_ENCRYPTION_KEY` Wrangler secret and are released only to an
approved Ed25519 bridge bound to that config. The binary does not read
`creds.json` or `sip.json` and never persists fetched credentials.

Generate the encryption master once with `openssl rand -base64 32` and install
it with `wrangler secret put SIP_SECRET_ENCRYPTION_KEY`. Existing SIP
ciphertext continues to use the same key.

# Multiple SIP connections

One IVR config can expose the same function graph through any number of SIP
connections. An endpoint sends every call to one menu. A trunk matches the
called number exactly and can have one catch-all route. Each connection owns a
dedicated signaling port; registered connections may use port `0` for automatic
allocation, while inbound-only trunks require a fixed port and source CIDRs.
Automatically allocated ports remain attached to their connection IDs across
hot reloads, so inserting or reordering another connection does not restart
healthy listeners.

```toml
[sip]
max_concurrent_calls = 20
record_path = "files/recording"

[[sip.connection]]
id = "support"
name = "Support endpoint"
kind = "endpoint"
registration = "registered"
server = "pbx.example.com:5060"
extension = "100"
entrypoint = "support-menu"

[[sip.connection]]
id = "public-trunk"
name = "Public numbers"
kind = "trunk"
registration = "inbound"
username = "asterisk"
transport = "udp"
local_port = 5062
allowed_cidrs = ["192.0.2.0/24"]

[[sip.connection.route]]
id = "sales-number"
number = "+461234567"
entrypoint = "sales-menu"

[[sip.connection.route]]
id = "trunk-fallback"
catch_all = true
entrypoint = "main"
```

For that inbound listener, Asterisk is the SIP client and must attach the same
username/password after the IVR's `401` challenge:

```ini
[benis-ivr-auth]
type=auth
auth_type=userpass
realm=*
username=asterisk
password=replace-with-the-password-entered-in-the-web-editor

[benis-ivr-aor]
type=aor
contact=sip:192.0.2.20:5062

[benis-ivr]
type=endpoint
transport=transport-udp
aors=benis-ivr-aor
outbound_auth=benis-ivr-auth
disallow=all
allow=ulaw,alaw
dtmf_mode=rfc4733
direct_media=no
```

Dial with `Dial(PJSIP/${EXTEN}@benis-ivr)` so the Request-URI user reaches the
trunk route matcher.

The web editor visualizes these as input nodes. It also shows registration and
listener health per IVR instance, with the latest 50 events retained for seven
days by the existing ConfigBroker Durable Object. Current state is replayed
whenever a runtime reconnects, so a WebSocket outage cannot permanently leave
the editor stale.

Routing-only changes apply to new calls without interrupting calls already in
progress. Changes to a listener's transport, port, external IP, registration
mode, server, domain, or SIP identity recreate that listener. Same-port changes
and port swaps drain affected calls first and start the newest saved
configuration automatically. A failure on one connection leaves its previous
working configuration in place and does not roll healthy connections back.

Every SIP connection has an encrypted password. `registered` connections use
it when the upstream PBX challenges REGISTER; `inbound` connections challenge
each INVITE with SIP Digest authentication and also require the source to match
`allowed_cidrs`.

## Bridge-only rollout

Apply all D1 migrations and deploy the Worker/UI first. Enter the config's
runtime credentials and SIP passwords, copy its registration ID, and start the
new binary. Compare the printed fingerprint with the pending row in the
Registrations tab before approving it. Stop the corresponding old SIP runtime
before the newly approved bridge takes ownership of its listeners. After every
bridge is active, old local credential files and the obsolete shared bearer
secret can be removed.

# Recoding
To get recording to work, create in the files a directory called "recoding".

# Running

The only runtime startup mode uses a config registration ID:

```sh
go run benis-phone.go -register 123e4567-e89b-42d3-a456-426614174000
```

The first run creates a private Ed25519 identity in the OS user config
directory and waits for web approval. Later runs use the same command and
identity. Override the exact identity path with `-bridge-identity`; delete that
identity and exit with `-reset-bridge`:

```sh
go run benis-phone.go -register 123e4567-e89b-42d3-a456-426614174000 -reset-bridge
```
