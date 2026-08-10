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

# Credentials
Create a dir called "creds" in the root, then create a file called creds.json, the file should look like this:

```
{
        "S3": {
                "Key": "xxx",
                "Secret": "xxx"
        },
        "Polly": {
                "Key": "xxx",
                "Secret": "xxx"
        },
        "Backend": {
                "Username": "xxx",
                "Password": "xxx"
        },
        "Trafiklab": "xxx",
        "Systemet": "xxx",
        "MediaServer": "xxx",
        "HTTPServerAuth": {
                "Username": "xxx",
                "Password": "xxx"
        }
}
```

SIP passwords are not stored in that file when using the web editor. They are
entered per SIP connection, encrypted by the Worker with the
`SIP_SECRET_ENCRYPTION_KEY` Wrangler secret, and delivered only to an IVR that
has the bearer token. Generate a key with `openssl rand -base64 32` and install
it with `wrangler secret put SIP_SECRET_ENCRYPTION_KEY`.

For `-source=file`, put passwords in `creds/sip.json`, keyed by the stable
connection ID:

```json
{
  "primary": "the-sip-password"
}
```

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

Passwords only belong to `registered` connections. Switching a connection to
`inbound` deletes its encrypted password during the same save and excludes it
from runtime bundles.

## Multi-SIP rollout

This is an intentional configuration migration: the new runtime rejects the
old single `[sip]` shape. For a remote deployment, stop the old IVR instances,
apply D1 migration `0002_sip_secrets.sql`, set the encryption key, and deploy
the Worker/UI. Open each legacy config in the editor, review its prefilled SIP
connection, enter its password, and save it. The new IVR instances can then be
started with the migrated configs. Keeping the old instances stopped during
the config saves prevents them from hot-loading a shape they do not understand.

# Recoding
To get recording to work, create in the files a directory called "recoding".

# Running

Remote mode is the default and requires a config name:

```sh
./benis-phone -config simonstorp
```

For local development, use a file containing a registered or inbound SIP
connection. An inbound connection replaces the old direct-call debug flags:

```sh
./benis-phone -source=file -def configurations/atp.toml -s3=false
```

Call the configured listener from a SIP client whose source address is covered
by `allowed_cidrs`. Use `-sip-secrets` when registered file-mode connections
store their password somewhere other than `creds/sip.json`.
