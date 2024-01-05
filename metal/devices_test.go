package metal

import (
	"context"
	"fmt"
	"strings"
	"testing"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"github.com/google/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cpapi "k8s.io/cloud-provider/api"
)

// testNode provides a simple Node object satisfying the lookup requirements of InstanceMetadata()
func testNodeWithIP(providerID, nodeName, nodeIP string) *v1.Node {
	node := testNode(providerID, nodeName)
	if nodeIP != "" {
		node.Annotations = map[string]string{
			cpapi.AnnotationAlphaProvidedIPAddr: nodeIP,
		}
	}
	return node
}

func testNode(providerID, nodeName string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Spec: v1.NodeSpec{
			ProviderID: providerID,
		},
	}
}

func TestNodeAddresses(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	if inst == nil {
		t.Fatal("inst is nil")
	}
	devName := testGetNewDevName()
	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE

	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}
	server.DeviceStore[uid] = dev
	project := server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, dev)
	server.ProjectStore[vc.config.ProjectID] = project

	// update the addresses on the device; normally created by Equinix Metal itself
	networks := []metal.IPAssignment{
		testCreateAddress(false, false), // private ipv4
		testCreateAddress(false, true),  // public ipv4
		testCreateAddress(true, true),   // public ipv6
	}
	kubeletNodeIP := testCreateAddress(false, false)
	dev.IpAddresses = networks

	validAddresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: devName},
		{Type: v1.NodeInternalIP, Address: networks[0].GetAddress()},
		{Type: v1.NodeExternalIP, Address: networks[1].GetAddress()},
	}

	validAddressesWithProvidedIP := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: devName},
		{Type: v1.NodeInternalIP, Address: kubeletNodeIP.GetAddress()},
		{Type: v1.NodeInternalIP, Address: networks[0].GetAddress()},
		{Type: v1.NodeExternalIP, Address: networks[1].GetAddress()},
	}

	tests := []struct {
		testName  string
		node      *v1.Node
		addresses []v1.NodeAddress
		err       error
	}{
		{"empty node name", testNode("", ""), nil, fmt.Errorf("node name cannot be empty")},
		{"instance not found", testNode("", nodeName), nil, fmt.Errorf("instance not found")},
		{"unknown name", testNode("equinixmetal://"+randomID, nodeName), nil, fmt.Errorf("instance not found")},
		{"valid both", testNode("equinixmetal://"+dev.GetId(), devName), validAddresses, nil},
		{"valid provider id", testNode("equinixmetal://"+dev.GetId(), nodeName), validAddresses, nil},
		{"valid node name", testNode("", devName), validAddresses, nil},
		{"with node IP", testNodeWithIP("equinixmetal://"+dev.GetId(), nodeName, kubeletNodeIP.GetAddress()), validAddressesWithProvidedIP, nil},
	}

	for i, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var addresses []v1.NodeAddress

			md, err := inst.InstanceMetadata(context.TODO(), tt.node)
			if md != nil {
				addresses = md.NodeAddresses
			}
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
			case !compareAddresses(addresses, tt.addresses):
				t.Errorf("%d: mismatched addresses, actual %v expected %v", i, addresses, tt.addresses)
			}
		})
	}
}

func TestNodeAddressesByProviderID(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	devName := testGetNewDevName()
	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE
	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}

	server.DeviceStore[uid] = dev
	project := server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, dev)
	server.ProjectStore[vc.config.ProjectID] = project

	// update the addresses on the device; normally created by Equinix Metal itself
	networks := []metal.IPAssignment{
		testCreateAddress(false, false), // private ipv4
		testCreateAddress(false, true),  // public ipv4
		testCreateAddress(true, true),   // public ipv6
	}
	dev.IpAddresses = networks

	validAddresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: devName},
		{Type: v1.NodeInternalIP, Address: networks[0].GetAddress()},
		{Type: v1.NodeExternalIP, Address: networks[1].GetAddress()},
	}

	tests := []struct {
		testName  string
		id        string
		addresses []v1.NodeAddress
		err       error
	}{
		{"empty ID", "", nil, fmt.Errorf("instance not found")},
		{"invalid format", randomID, nil, fmt.Errorf("instance not found")},
		{"not equinixmetal", "aws://" + randomID, nil, fmt.Errorf("provider name from providerID should be equinixmetal")},
		{"unknown ID", "equinixmetal://" + randomID, nil, fmt.Errorf("instance not found")},
		{"valid prefix", fmt.Sprintf("equinixmetal://%s", dev.GetId()), validAddresses, nil},
		{"valid legacy prefix", fmt.Sprintf("packet://%s", dev.GetId()), validAddresses, nil},
		{"valid without prefix", dev.GetId(), validAddresses, nil},
	}

	for i, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var addresses []v1.NodeAddress

			md, err := inst.InstanceMetadata(context.TODO(), testNode(tt.id, nodeName))
			if md != nil {
				addresses = md.NodeAddresses
			}
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
			case !compareAddresses(addresses, tt.addresses):
				t.Errorf("%d: mismatched addresses, actual %v expected %v", i, addresses, tt.addresses)
			}
		})
	}
}

/*
	func TestInstanceID(t *testing.T) {
		vc, server := testGetValidCloud(t, "")
		inst, _ := vc.InstancesV2()
		devName := testGetNewDevName()
		facility, _ := testGetOrCreateValidZone(validZoneName, validZoneCode, backend)
		plan, _ := testGetOrCreateValidPlan(validPlanName, validPlanSlug, backend)
		dev, _ := backend.CreateDevice(projectID, devName, plan, facility)

		tests := []struct {
			name string
			id   string
			err  error
		}{
			{"", "", fmt.Errorf("node name cannot be empty")},          // empty name
			{"thisdoesnotexist", "", fmt.Errorf("instance not found")}, // unknown name
			{devName, dev.GetId(), nil},                                     // valid
		}

		for i, tt := range tests {
			var id string
			md, err := inst.InstanceMetadata(context.TODO(), testNode(tt.id, nodeName))
			if md != nil {
				id, err = deviceIDFromProviderID(md.ProviderID)
			}

			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("%d: mismatched errors, actual %q expected %q", i, err, tt.err)
			case id != tt.id:
				t.Errorf("%d: mismatched id, actual %v expected %v", i, id, tt.id)
			}
		}
	}
*/
func TestInstanceType(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	devName := testGetNewDevName()

	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE
	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}
	server.DeviceStore[uid] = dev
	project := server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, dev)
	server.ProjectStore[vc.config.ProjectID] = project

	privateIP := "10.1.1.2"
	publicIP := "25.50.75.100"
	trueBool := true
	ipv4 := int32(metal.IPADDRESSADDRESSFAMILY__4)
	dev.IpAddresses = append(dev.IpAddresses, []metal.IPAssignment{
		{
			Address:       &privateIP,
			Management:    &trueBool,
			AddressFamily: &ipv4,
		},
		{
			Address:       &publicIP,
			Public:        &trueBool,
			AddressFamily: &ipv4,
		},
	}...)

	tests := []struct {
		testName string
		name     string
		plan     string
		err      error
	}{
		{"empty name", "", "", fmt.Errorf("instance not found")},
		{"unknown name", randomID, "", fmt.Errorf("instance not found")},
		{"valid", "equinixmetal://" + dev.GetId(), dev.Plan.GetSlug(), nil},
	}

	for i, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var plan string
			md, err := inst.InstanceMetadata(context.TODO(), testNode(tt.name, nodeName))
			if md != nil {
				plan = md.InstanceType
			}
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
			case plan != tt.plan:
				t.Errorf("%d: mismatched id, actual %v expected %v", i, plan, tt.plan)
			}
		})
	}
}

func TestInstanceZone(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	devName := testGetNewDevName()
	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE
	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}

	privateIP := "10.1.1.2"
	publicIP := "25.50.75.100"

	metroId := "123"
	regionCode := validRegionCode
	regionName := validRegionName
	country := "Country"
	metro := &metal.DeviceMetro{Id: &metroId, Code: &regionCode, Name: &regionName, Country: &country}
	dev.Metro = metro

	trueBool := true
	ipv4 := int32(metal.IPADDRESSADDRESSFAMILY__4)
	dev.IpAddresses = append(dev.IpAddresses, []metal.IPAssignment{
		{
			Address:       &privateIP,
			Management:    &trueBool,
			AddressFamily: &ipv4,
		},
		{
			Address:       &publicIP,
			Public:        &trueBool,
			AddressFamily: &ipv4,
		},
	}...)

	server.DeviceStore[uid] = dev
	project := server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, dev)
	server.ProjectStore[vc.config.ProjectID] = project

	tests := []struct {
		testName string
		name     string
		region   string
		err      error
	}{
		{"empty name", "", "", fmt.Errorf("instance not found")},
		{"unknown name", randomID, "", fmt.Errorf("instance not found")},
		{"valid", "equinixmetal://" + dev.GetId(), validRegionCode, nil},
	}

	for i, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var region string
			md, err := inst.InstanceMetadata(context.TODO(), testNode(tt.name, nodeName))
			if md != nil {
				region = md.Region
			}
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
			case region != tt.region:
				t.Errorf("%d: mismatched region, actual %v expected %v", i, region, tt.region)
			}
		})
	}
}

/*
func TestInstanceTypeByProviderID(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.Instances()
	devName := testGetNewDevName()
	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE
	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}

	tests := []struct {
		id   string
		plan string
		err  error
	}{
		{"", "", fmt.Errorf("providerID cannot be empty")},                                            // empty name
		{randomID, "", fmt.Errorf("instance not found")},                                              // invalid format
		{"aws://" + randomID, "", fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetalk
		{"equinixmetal://" + randomID, "", fmt.Errorf("instance not found")},                          // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.GetId()), dev.Plan.Name, nil},                                // valid
		{fmt.Sprintf("packet://%s", dev.GetId()), dev.Plan.Name, nil},                                      // valid
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
	vc, _ := testGetValidCloud(t, "")
	inst, _ := vc.Instances()
	err := inst.AddSSHKeyToAllInstances(nil, "", nil)
	if err != cloudprovider.NotImplemented {
		t.Errorf("mismatched error: expected %v received %v", cloudprovider.NotImplemented, err)
	}
}

func TestCurrentNodeName(t *testing.T) {
	vc, _ := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	var (
		devName       = testGetNewDevName()
		expectedError error
		expectedName  = types.NodeName(devName)
	)

	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE
	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}

	md, err := inst.InstanceMetadata(context.TODO(), testNode("equinixmetal://"+dev.GetId(), nodeName))

	if err != expectedError {
		t.Errorf("mismatched errors, actual %v expected %v", err, expectedError)
	}
	if md. != expectedName {
		t.Errorf("mismatched nodename, actual %v expected %v", nn, expectedName)
	}
}

*/

func TestInstanceExistsByProviderID(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	devName := testGetNewDevName()
	uid := uuid.New().String()
	state := metal.DEVICESTATE_ACTIVE
	dev := &metal.Device{
		Id:       &uid,
		Hostname: &devName,
		State:    &state,
		Plan:     metal.NewPlan(),
	}

	server.DeviceStore[uid] = dev
	project := server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, dev)
	server.ProjectStore[vc.config.ProjectID] = project

	tests := []struct {
		id     string
		exists bool
		err    error
	}{
		{"", false, fmt.Errorf("providerID cannot be empty")}, // empty name
		{randomID, false, nil},                                // invalid format
		{"aws://" + randomID, false, fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://" + randomID, false, nil},                                                       // unknown ID
		{fmt.Sprintf("equinixmetal://%s", dev.GetId()), true, nil},                                       // valid
		{fmt.Sprintf("packet://%s", dev.GetId()), true, nil},                                             // valid
		{dev.GetId(), true, nil},                                                                         // valid
	}

	for i, tt := range tests {
		exists, err := inst.InstanceExists(context.TODO(), testNode(tt.id, nodeName))
		switch {
		case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
			t.Errorf("%d: mismatched errors, actual %v expected %v", i, err, tt.err)
		case exists != tt.exists:
			t.Errorf("%d: mismatched exists, actual %v expected %v", i, exists, tt.exists)
		}
	}
}

func TestInstanceShutdownByProviderID(t *testing.T) {
	vc, server := testGetValidCloud(t, "")
	inst, _ := vc.InstancesV2()
	devName := testGetNewDevName()
	activeDevUid := uuid.New().String()
	activeState := metal.DEVICESTATE_ACTIVE
	devActive := &metal.Device{
		Id:       &activeDevUid,
		Hostname: &devName,
		State:    &activeState,
		Plan:     metal.NewPlan(),
	}
	server.DeviceStore[activeDevUid] = devActive
	project := server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, devActive)
	server.ProjectStore[vc.config.ProjectID] = project

	inactiveDevUid := uuid.New().String()
	inactiveState := metal.DEVICESTATE_INACTIVE
	devInactive := &metal.Device{
		Id:       &inactiveDevUid,
		Hostname: &devName,
		State:    &inactiveState,
		Plan:     metal.NewPlan(),
	}
	server.DeviceStore[inactiveDevUid] = devInactive
	project = server.ProjectStore[vc.config.ProjectID]
	project.Devices = append(project.Devices, devInactive)
	server.ProjectStore[vc.config.ProjectID] = project

	tests := []struct {
		id   string
		down bool
		err  error
	}{
		{"", false, fmt.Errorf("providerID cannot be empty")},                                            // empty name
		{randomID, false, fmt.Errorf("instance not found")},                                              // invalid format
		{"aws://" + randomID, false, fmt.Errorf("provider name from providerID should be equinixmetal")}, // not equinixmetal
		{"equinixmetal://" + randomID, false, fmt.Errorf("instance not found")},                          // unknown ID
		{fmt.Sprintf("equinixmetal://%s", devActive.GetId()), false, nil},                                // valid
		{fmt.Sprintf("packet://%s", devActive.GetId()), false, nil},                                      // valid
		{devActive.GetId(), false, nil},                                                                  // valid
		{fmt.Sprintf("equinixmetal://%s", devInactive.GetId()), true, nil},                               // valid
		{fmt.Sprintf("packet://%s", devInactive.GetId()), true, nil},                                     // valid
		{devInactive.GetId(), true, nil},                                                                 // valid
	}

	for i, tt := range tests {
		down, err := inst.InstanceShutdown(context.TODO(), testNode(tt.id, nodeName))
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
