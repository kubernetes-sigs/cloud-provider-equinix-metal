package metal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/packethost/packngo"
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
	facility, _ := testGetOrCreateValidZone(validZoneName, validZoneCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)
	dev.Metro = &packngo.Metro{ID: "123", Code: validRegionCode, Name: "Metro", Country: "Country"}

	tests := []struct {
		providerID    string
		region        string
		failureDomain string
		err           error
	}{
		{"", "", "", fmt.Errorf("providerID cannot be empty")},                                            // empty ID
		{randomID, "", "", fmt.Errorf("instance not found")},                                              // invalid format
		{"aws://" + randomID, "", "", fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://" + randomID, "", "", fmt.Errorf("instance not found")},                          // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.ID), validRegionCode, validZoneCode, nil},                   // valid
	}

	zones, _ := vc.Zones()
	for i, tt := range tests {
		zone, err := zones.GetZoneByProviderID(nil, tt.providerID)
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case zone.Region != tt.region:
			t.Errorf("%d: mismatched region, actual %v expected %v", i, zone.Region, tt.region)
		case zone.FailureDomain != tt.failureDomain:
			t.Errorf("%d: mismatched failureDomain, actual %v expected %v", i, zone.FailureDomain, tt.failureDomain)
		}
	}
}

func TestGetZoneByNodeName(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidZone(validZoneName, validZoneCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)
	dev.Metro = &packngo.Metro{ID: "123", Code: validRegionCode, Name: "Metro", Country: "Country"}

	tests := []struct {
		name          string
		region        string
		failureDomain string
		err           error
	}{
		{"", "", "", fmt.Errorf("node name cannot be empty")},          // empty name
		{"thisdoesnotexist", "", "", fmt.Errorf("instance not found")}, // unknown name
		{devName, validRegionCode, validZoneCode, nil},                 // valid
	}

	zones, _ := vc.Zones()
	for i, tt := range tests {
		zone, err := zones.GetZoneByNodeName(nil, types.NodeName(tt.name))
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case zone.Region != tt.region:
			t.Errorf("%d: mismatched region, actual %v expected %v", i, zone.Region, tt.region)
		case zone.FailureDomain != tt.failureDomain:
			t.Errorf("%d: mismatched failureDomain, actual %v expected %v", i, zone.FailureDomain, tt.failureDomain)
		}

	}
}
