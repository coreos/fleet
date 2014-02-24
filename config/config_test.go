package config

import (
	"flag"
	"testing"
)

func TestConfigMetadata(t *testing.T) {
	raw := "foo=bar, ping=pong"
	cfg := Config{RawMetadata: raw}
	metadata := cfg.Metadata()

	if len(metadata) != 2 {
		t.Errorf("Parsed %d keys, expected 1", len(metadata))
	}

	if metadata["foo"] != "bar" {
		t.Errorf("Incorrect value '%s' of key 'foo', expected 'bar'", metadata["foo"])
	}

	if metadata["ping"] != "pong" {
		t.Errorf("Incorrect value '%s' of key 'ping', expected 'pong'", metadata["ping"])
	}
}

func TestConfigMetadataNotSet(t *testing.T) {
	cfg := Config{}
	metadata := cfg.Metadata()

	if len(metadata) != 0 {
		t.Errorf("Parsed %d keys, expected 0", len(metadata))
	}
}

func TestConfigUpdateLoggingFlags(t *testing.T) {
	flagset := flag.NewFlagSet("fleet-testing", flag.ContinueOnError)
	flagset.Bool("logtostderr", false, "")
	flagset.Int("v", 0, "")

	cfg := Config{Verbosity: 2}

	UpdateLoggingFlagsFromConfig(flagset, &cfg)
}
