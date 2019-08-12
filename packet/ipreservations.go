package packet

import (
	"github.com/packethost/packngo"
)

// ipReservationByTags given a set of packngo.IPAddressReservation and a set of tags, find
// the reservations that have those tags
func ipReservationByTags(targetTags []string, ips []packngo.IPAddressReservation) *packngo.IPAddressReservation {
	// cycle through the IPs, looking for one that matches ours
ips:
	for i, ip := range ips {
		tagMatches := map[string]bool{}
		for _, t := range targetTags {
			tagMatches[t] = false
		}
		for _, tag := range ip.Tags {
			if _, ok := tagMatches[tag]; ok {
				tagMatches[tag] = true
			}
		}
		// does this IP match?
		for _, v := range tagMatches {
			// any missing tag says no match
			if !v {
				continue ips
			}
		}
		// if we made it here, we matched
		return &ips[i]
	}
	// if we made it here, nothing matched
	return nil
}
