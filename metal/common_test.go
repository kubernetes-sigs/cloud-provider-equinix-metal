package metal

import (
	"fmt"
	"math/rand"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
	"github.com/google/uuid"
	randomdata "github.com/pallinder/go-randomdata"
)

var randomID = uuid.New().String()

// get a unique name
func testGetNewDevName() string {
	return fmt.Sprintf("device-%d", rand.Intn(1000))
}

func testCreateAddress(ipv6, public bool) metal.IPAssignment {
	family := int32(metal.IPADDRESSADDRESSFAMILY__4)
	if ipv6 {
		family = int32(metal.IPADDRESSADDRESSFAMILY__6)
	}
	ipaddr := ""
	if ipv6 {
		ipaddr = randomdata.IpV6Address()
	} else {
		ipaddr = randomdata.IpV4Address()
	}
	address := metal.IPAssignment{
		Address:       &ipaddr,
		Public:        &public,
		AddressFamily: &family,
	}
	return address
}
