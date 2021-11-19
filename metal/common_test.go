package metal

import (
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/packethost/packet-api-server/pkg/store"
	"github.com/packethost/packngo"
	"github.com/packethost/packngo/metadata"
	randomdata "github.com/pallinder/go-randomdata"
)

var randomID = uuid.New().String()

// find an ewr1 region or create it
func testGetOrCreateValidRegion(name, code string, backend store.DataStore) (*packngo.Facility, error) {
	facility, err := backend.GetFacilityByCode(code)
	if err != nil {
		return nil, err
	}
	// if we already have it, use it
	if facility != nil {
		return facility, nil
	}
	// we do not have it, so create it
	return backend.CreateFacility(name, code)
}

// find an ewr1 region or create it
func testGetOrCreateValidPlan(name, slug string, backend store.DataStore) (*packngo.Plan, error) {
	plan, err := backend.GetPlanBySlug(slug)
	if err != nil {
		return nil, err
	}
	// if we already have it, use it
	if plan != nil {
		return plan, nil
	}
	// we do not have it, so create it
	return backend.CreatePlan(slug, name)
}

// get a unique name
func testGetNewDevName() string {
	return fmt.Sprintf("device-%d", rand.Intn(1000))
}

func testCreateAddress(ipv6, public bool) *packngo.IPAddressAssignment {
	family := metadata.IPv4
	if ipv6 {
		family = metadata.IPv6
	}
	ipaddr := ""
	if ipv6 {
		ipaddr = randomdata.IpV6Address()
	} else {
		ipaddr = randomdata.IpV4Address()
	}
	address := &packngo.IPAddressAssignment{
		IpAddressCommon: packngo.IpAddressCommon{
			Address:       ipaddr,
			Public:        public,
			AddressFamily: int(family),
		},
	}
	return address
}
