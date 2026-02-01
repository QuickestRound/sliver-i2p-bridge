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
    # Enable SAM bridge if not already enabled
    if ! grep -q "clientApp.3.startOnLoad=true" "$I2P_CONFIG" 2>/dev/null; then
        echo "[*] Enabling SAM bridge in I2P config..."
        sudo sed -i 's/clientApp.3.startOnLoad=false/clientApp.3.startOnLoad=true/' "$I2P_CONFIG" || true
    fi
else
    echo "[!] I2P config not found at $I2P_CONFIG"
    echo "[!] You may need to start I2P once to generate config, then re-run this script"
fi

# Install Go (if not present)
echo "[*] Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "[*] Installing Go..."
    GO_VERSION="1.21.6"
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
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
