#!/bin/bash
# Whonix-Workstation installation script for sliver-i2p-bridge
# Run this inside your Whonix-Workstation VM

set -e

echo "[*] sliver-i2p-bridge Whonix Installer"
echo "[*] =================================="
echo ""

# Check if running in Whonix
if [ ! -f /usr/share/anon-gw-base-files/gateway ]; then
    if [ ! -f /usr/share/anon-ws-base-files/workstation ]; then
        echo "[!] Warning: This doesn't appear to be a Whonix system"
        echo "[!] Continuing anyway..."
    fi
fi

# Update package lists
echo "[*] Updating package lists..."
sudo apt-get update -qq

# Install I2P
echo "[*] Installing I2P..."
if ! command -v i2prouter &> /dev/null; then
    # Add I2P repository
    sudo apt-get install -y apt-transport-https curl
    
    # Import I2P signing key
    curl -s https://geti2p.net/_static/i2p-debian-repo.key.asc | sudo apt-key add -
    
    # Add repository
    echo "deb https://deb.i2p2.de/ stable main" | sudo tee /etc/apt/sources.list.d/i2p.list
    
    sudo apt-get update -qq
    sudo apt-get install -y i2p i2p-keyring
else
    echo "[+] I2P already installed"
fi

# Configure I2P SAM bridge
echo "[*] Configuring I2P SAM bridge..."
I2P_CONFIG="/var/lib/i2p/i2p-config/clients.config"

if [ -f "$I2P_CONFIG" ]; then
    # Enable SAM bridge - search for SAM class rather than hardcoded clientApp number
    # SAM is identified by its class name: net.i2p.sam.SAMBridge
    if grep -q "SAMBridge" "$I2P_CONFIG" 2>/dev/null; then
        echo "[*] Enabling SAM bridge in I2P config..."
        
        # Find the SAM clientApp ID using POSIX-compatible grep -E (not -P)
        # Skip commented lines (#) and look for: clientApp.N.main=net.i2p.sam.SAMBridge
        SAM_ID=$(grep -v '^\s*#' "$I2P_CONFIG" | grep -E 'clientApp\.[0-9]+\s*\.\s*main\s*=\s*net\.i2p\.sam\.SAMBridge' | sed -E 's/^(clientApp\.[0-9]+)\..*$/\1/' | head -1)
        
        if [ -n "$SAM_ID" ]; then
            # Enable SAM bridge (handle optional spaces around = in both search and replace)
            sudo sed -i -E "s/(${SAM_ID}[[:space:]]*\.[[:space:]]*startOnLoad[[:space:]]*=[[:space:]]*)false/\1true/" "$I2P_CONFIG" || true
            echo "[+] SAM bridge enabled (${SAM_ID})"
            
            # Verify the change was applied
            if grep -q "${SAM_ID}.startOnLoad=true" "$I2P_CONFIG"; then
                echo "[+] Verified: SAM bridge is set to start on load"
            else
                echo "[!] Warning: Could not verify SAM config change - check manually"
                echo "[!] Expected: ${SAM_ID}.startOnLoad=true"
            fi
        else
            echo "[!] Could not find SAM clientApp ID in config"
            echo "[!] Please manually enable SAM bridge in I2P router console (http://127.0.0.1:7657)"
        fi
    else
        echo "[!] SAM bridge not found in config - may need manual configuration"
    fi
else
    echo "[!] I2P config not found at $I2P_CONFIG"
    echo "[!] You may need to start I2P once to generate config, then re-run this script"
fi

# Install Go (if not present)
echo "[*] Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "[*] Installing Go..."
    # Get latest stable Go version dynamically
    GO_VERSION=$(curl -s 'https://go.dev/VERSION?m=text' | head -1 | sed 's/go//')
    if [ -z "$GO_VERSION" ]; then
        # Fallback to known stable version
        GO_VERSION="1.22.5"
        echo "[!] Could not fetch latest Go version, using fallback: $GO_VERSION"
    else
        echo "[*] Latest Go version: $GO_VERSION"
    fi
    
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
    if [ $? -ne 0 ]; then
        echo "[!] Failed to download Go $GO_VERSION"
        exit 1
    fi
    
    # Verify SHA256 checksum (download expected hash from go.dev)
    # SECURITY: Fail-closed - if we can't verify, we abort
    echo "[*] Verifying SHA256 checksum..."
    EXPECTED_HASH=$(curl -s "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz.sha256")
    if [ -z "$EXPECTED_HASH" ]; then
        echo "[!] SECURITY: Could not fetch checksum - aborting"
        echo "[!] This could indicate a network issue or MITM attack"
        rm -f /tmp/go.tar.gz
        exit 1
    fi
    ACTUAL_HASH=$(sha256sum /tmp/go.tar.gz | awk '{print $1}')
    if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
        echo "[!] SECURITY WARNING: SHA256 checksum mismatch!"
        echo "[!] Expected: $EXPECTED_HASH"
        echo "[!] Got:      $ACTUAL_HASH"
        echo "[!] Aborting to prevent potential MITM attack"
        rm -f /tmp/go.tar.gz
        exit 1
    fi
    echo "[+] Checksum verified!"
    
    sudo rm -rf /usr/local/go  # Remove old version to prevent mixed files
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    
    # Add to path
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
    export PATH=$PATH:/usr/local/go/bin
else
    echo "[+] Go already installed: $(go version)"
fi

# Build sliver-i2p-bridge
echo "[*] Building sliver-i2p-bridge..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Download dependencies
go mod tidy

# Build binary
CGO_ENABLED=0 go build -ldflags="-s -w" -o sliver-i2p-bridge ./cmd/sliver-i2p-bridge

# Install binary
echo "[*] Installing binary..."
sudo cp sliver-i2p-bridge /usr/local/bin/
sudo chmod +x /usr/local/bin/sliver-i2p-bridge

# Install systemd services
echo "[*] Installing systemd services..."
sudo cp systemd/*.service /etc/systemd/system/ 2>/dev/null || true
sudo systemctl daemon-reload

# Create data directory
echo "[*] Creating data directory..."
sudo mkdir -p /var/lib/sliver-i2p-bridge
sudo chmod 700 /var/lib/sliver-i2p-bridge

echo ""
echo "[+] Installation complete!"
echo ""
echo "[*] Next steps:"
echo "    1. Start I2P:     sudo systemctl start i2p"
echo "    2. Wait for I2P to bootstrap (check http://127.0.0.1:7657)"
echo "    3. Start Sliver:  sliver-server"
echo "    4. Create HTTPS listener in Sliver: https -L 127.0.0.1 -l 8443"
echo "    5. Start bridge:  sliver-i2p-bridge start --sliver-port 8443"
echo ""
echo "[*] Or use systemd:"
echo "    sudo systemctl enable --now i2p"
echo "    sudo systemctl start sliver-i2p-bridge"
