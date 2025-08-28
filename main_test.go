package main

import (
	"testing"

	"sigs.k8s.io/cloud-provider-equinix-metal/metal"
)

func TestProviderName(t *testing.T) {
	if metal.ProviderName != "equinixmetal" {
		t.Errorf("expected provider name to be 'equinixmetal', got '%s'", metal.ProviderName)
	}
}

func TestCloudInitializer(t *testing.T) {
	_ = cloudInitializer
}
