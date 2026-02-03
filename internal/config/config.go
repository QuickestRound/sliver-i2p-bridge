package config

// Config holds all bridge configuration
type Config struct {
	// Sliver connection settings
	SliverHost string
	SliverPort int

	// I2P SAM bridge settings
	SAMHost string
	SAMPort int

	// Key persistence
	PersistKeys bool
	KeyPath     string

	// TLS settings for Sliver connection
	SkipTLSVerify bool
	SliverCA      string // Optional path to CA cert for TLS verification
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		SliverHost:    "127.0.0.1",
		SliverPort:    8443,
		SAMHost:       "127.0.0.1",
		SAMPort:       7656,
		PersistKeys:   false,
		KeyPath:       "destination.keys",
		SkipTLSVerify: true, // Sliver uses self-signed certs by default
	}
}
