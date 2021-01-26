package metal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/packethost/packngo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

func TestNodeAddresses(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)
	// update the addresses on the device; normally created by Equinix Metal itself
	networks := []*packngo.IPAddressAssignment{
		testCreateAddress(false, false), // private ipv4
		testCreateAddress(false, true),  // public ipv4
		testCreateAddress(true, true),   // public ipv6
	}
	dev.Network = networks
	err := backend.UpdateDevice(dev.ID, dev)
	if err != nil {
		t.Fatalf("unable to update inactive device: %v", err)
	}

	validAddresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: devName},
		{Type: v1.NodeInternalIP, Address: networks[0].Address},
		{Type: v1.NodeExternalIP, Address: networks[1].Address},
	}

	tests := []struct {
		name      string
		addresses []v1.NodeAddress
		err       error
	}{
		{"", nil, fmt.Errorf("node name cannot be empty")},          // empty name
		{"thisdoesnotexist", nil, fmt.Errorf("instance not found")}, // unknown name
		{devName, validAddresses, nil},                              // valid
	}

	for i, tt := range tests {
		addresses, err := inst.NodeAddresses(nil, types.NodeName(tt.name))
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case !compareAddresses(addresses, tt.addresses):
			t.Errorf("%d: mismatched addresses, actual %v expected %v", i, addresses, tt.addresses)
		}
	}
}
func TestNodeAddressesByProviderID(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)
	// update the addresses on the device; normally created by Equinix Metal itself
	networks := []*packngo.IPAddressAssignment{
		testCreateAddress(false, false), // private ipv4
		testCreateAddress(false, true),  // public ipv4
		testCreateAddress(true, true),   // public ipv6
	}
	dev.Network = networks
	err := backend.UpdateDevice(dev.ID, dev)
	if err != nil {
		t.Fatalf("unable to update inactive device: %v", err)
	}

	validAddresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: devName},
		{Type: v1.NodeInternalIP, Address: networks[0].Address},
		{Type: v1.NodeExternalIP, Address: networks[1].Address},
	}

	tests := []struct {
		id        string
		addresses []v1.NodeAddress
		err       error
	}{
		{"", nil, fmt.Errorf("providerID cannot be empty")},                                           // empty ID
		{"foo-bar-abcdefg", nil, fmt.Errorf("instance not found")},                                    // invalid format
		{"aws://abcdef5667", nil, fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://acbdef-56788", nil, fmt.Errorf("instance not found")},                        // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.ID), validAddresses, nil},                               // valid
		{dev.ID, validAddresses, nil},                                                                 // valid
	}

	for i, tt := range tests {
		addresses, err := inst.NodeAddressesByProviderID(nil, tt.id)
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case !compareAddresses(addresses, tt.addresses):
			t.Errorf("%d: mismatched addresses, actual %v expected %v", i, addresses, tt.addresses)
		}
	}
}

func TestInstanceID(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)

	tests := []struct {
		name string
		id   string
		err  error
	}{
		{"", "", fmt.Errorf("node name cannot be empty")},          // empty name
		{"thisdoesnotexist", "", fmt.Errorf("instance not found")}, // unknown name
		{devName, dev.ID, nil},                                     // valid
	}

	for i, tt := range tests {
		id, err := inst.InstanceID(nil, types.NodeName(tt.name))
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case id != tt.id:
			t.Errorf("%d: mismatched id, actual %v expected %v", i, id, tt.id)
		}
	}
}

func TestInstanceType(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)

	tests := []struct {
		name string
		plan string
		err  error
	}{
		{"", "", fmt.Errorf("node name cannot be empty")},          // empty name
		{"thisdoesnotexist", "", fmt.Errorf("instance not found")}, // unknown name
		{devName, dev.Plan.Name, nil},                              // valid
	}

	for i, tt := range tests {
		plan, err := inst.InstanceType(nil, types.NodeName(tt.name))
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case plan != tt.plan:
			t.Errorf("%d: mismatched id, actual %v expected %v", i, plan, tt.plan)
		}
	}
}

func TestInstanceTypeByProviderID(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)

	tests := []struct {
		id   string
		plan string
		err  error
	}{
		{"", "", fmt.Errorf("providerID cannot be empty")},                                           // empty name
		{"foo-bar-abcdefg", "", fmt.Errorf("instance not found")},                                    // invalid format
		{"aws://abcdef5667", "", fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetalk
		{"equinixmetal://acbdef-56788", "", fmt.Errorf("instance not found")},                        // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.ID), dev.Plan.Name, nil},                               // valid
	}

	for i, tt := range tests {
		plan, err := inst.InstanceTypeByProviderID(nil, tt.id)
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case plan != tt.plan:
			t.Errorf("%d: mismatched id, actual %v expected %v", i, plan, tt.plan)
		}
	}
}

func TestAddSSHKeyToAllInstances(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	inst, _ := vc.Instances()
	err := inst.AddSSHKeyToAllInstances(nil, "", nil)
	if err != cloudprovider.NotImplemented {
		t.Errorf("mismatched error: expected %v received %v", cloudprovider.NotImplemented, err)
	}
}

func TestCurrentNodeName(t *testing.T) {
	vc, _ := testGetValidCloud(t)
	inst, _ := vc.Instances()
	var (
		name          = "foobar"
		expectedError error
		expectedName  = types.NodeName(name)
	)
	nn, err := inst.CurrentNodeName(nil, name)
	if err != expectedError {
		t.Errorf("mismatched errors, actual %v expected %v", err, expectedError)
	}
	if nn != expectedName {
		t.Errorf("mismatched nodename, actual %v expected %v", nn, expectedName)
	}
}

func TestInstanceExistsByProviderID(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	dev, _ := backend.CreateDevice(projectID, devName, plan, facility)

	tests := []struct {
		id     string
		exists bool
		err    error
	}{
		{"", false, fmt.Errorf("providerID cannot be empty")},                                           // empty name
		{"foo-bar-abcdefg", false, nil},                                                                 // invalid format
		{"aws://abcdef5667", false, fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://acbdef-56788", false, nil},                                                     // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.ID), true, nil},                                           // valid
		{dev.ID, true, nil}, // valid
	}

	for i, tt := range tests {
		exists, err := inst.InstanceExistsByProviderID(nil, tt.id)
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case exists != tt.exists:
			t.Errorf("%d: mismatched exists, actual %v expected %v", i, exists, tt.exists)
		}
	}
}

func TestInstanceShutdownByProviderID(t *testing.T) {
	vc, backend := testGetValidCloud(t)
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	facility, _ := testGetOrCreateValidRegion(validRegionName, validRegionCode, backend)
	plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
	devActive, _ := backend.CreateDevice(projectID, devName, plan, facility)
	devInactive, _ := backend.CreateDevice(projectID, devName, plan, facility)
	devInactive.State = "inactive"
	err := backend.UpdateDevice(devInactive.ID, devInactive)
	if err != nil {
		t.Fatalf("unable to update inactive device: %v", err)
	}

	tests := []struct {
		id   string
		down bool
		err  error
	}{
		{"", false, fmt.Errorf("providerID cannot be empty")},                                           // empty name
		{"foo-bar-abcdefg", false, fmt.Errorf("instance not found")},                                    // invalid format
		{"aws://abcdef5667", false, fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://acbdef-56788", false, fmt.Errorf("instance not found")},                        // unknown ID
		{fmt.Sprintf("equinixmetal://%s", devActive.ID), false, nil},                                    // valid
		{devActive.ID, false, nil},                                    // valid
		{fmt.Sprintf("equinixmetal://%s", devInactive.ID), true, nil}, // valid
		{devInactive.ID, true, nil},                                   // valid
	}

	for i, tt := range tests {
		down, err := inst.InstanceShutdownByProviderID(nil, tt.id)
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case down != tt.down:
			t.Errorf("%d: mismatched down, actual %v expected %v", i, down, tt.down)
		}
	}
}

func compareAddresses(a1, a2 []v1.NodeAddress) bool {
	switch {
	case (a1 == nil && a2 != nil) || (a1 != nil && a2 == nil):
		return false
	case a1 == nil && a2 == nil:
		return true
	case len(a1) != len(a2):
		return false
	default:
		for i := range a1 {
			if a1[i] != a2[i] {
				return false
			}
		}
		return true
	}

}
