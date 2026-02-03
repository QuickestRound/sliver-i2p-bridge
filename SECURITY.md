# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x     | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in sliver-i2p-bridge, please report it responsibly:

1. **DO NOT** open a public GitHub issue
2. Use GitHub's **private vulnerability reporting** feature:
   - Go to the Security tab → Report a vulnerability
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We will respond within 48 hours and work with you to address the issue.

## Security Considerations

This tool is designed for legitimate security testing. Users should:

- Only use with proper authorization
- Protect destination keys (`*.keys` files)
- Run in isolated environments (VMs, Whonix)
- Never expose SAM bridge to untrusted networks
- Use I2P's built-in security features

## Threat Model

sliver-i2p-bridge assumes:

- The operator controls the machine running the bridge
- I2P router is properly configured and bootstrapped
- Sliver server is running on localhost (not exposed)
- Network monitoring exists but cannot break I2P encryption

**Out of scope:**
- Attacks requiring physical access
- Compromise of I2P network itself
- Side-channel attacks on the host OS
