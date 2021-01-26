package metal

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

func TestGetZone(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	zones, _ := vc.Zones()
	zone, err := zones.GetZone(nil)
	var (
		expectedZone  = cloudprovider.Zone{}
		expectedError = cloudprovider.NotImplemented
	)
	if err != expectedError {
		t.Errorf("mismatched error: received %v instead of %v", err, expectedError)
	}
	if zone != expectedZone {
		t.Errorf("mismatched zone: received %v instead of %v", zone, expectedZone)
	}
}

func TestGetZoneByProviderID(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	// create a device
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)

	tests := []struct {
		providerID string
		region     string
		err        error
	}{
		{"", "", fmt.Errorf("providerID cannot be empty")},                                           // empty ID
		{"foo-bar-abcdefg", "", fmt.Errorf("instance not found")},                                    // invalid format
		{"aws://abcdef5667", "", fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://acbdef-56788", "", fmt.Errorf("instance not found")},                        // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.ID), validRegionCode, nil},                             // valid
	}

	zones, _ := vc.Zones()
	for i, tt := range tests {
		zone, err := zones.GetZoneByProviderID(nil, tt.providerID)
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case zone.Region != tt.region:
			t.Errorf("%d: mismatched zone, actual %v expected %v", i, zone.Region, tt.region)
		}
	}
}

func TestGetZoneByNodeName(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	backend.CreateDevice(projectID, devName, plan, facility)

	tests := []struct {
		name   string
		region string
		err    error
	}{
		{"", "", fmt.Errorf("node name cannot be empty")},          // empty name
		{"thisdoesnotexist", "", fmt.Errorf("instance not found")}, // unknown name
		{devName, validRegionCode, nil},                            // valid
	}

	zones, _ := vc.Zones()
	for i, tt := range tests {
		zone, err := zones.GetZoneByNodeName(nil, types.NodeName(tt.name))
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case zone.Region != tt.region:
			t.Errorf("%d: mismatched zone, actual %v expected %v", i, zone.Region, tt.region)
		}
	}
}
