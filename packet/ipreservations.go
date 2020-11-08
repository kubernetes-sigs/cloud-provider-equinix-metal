package packet

import (
	"github.com/packethost/packngo"
)

// ipReservationByAllTags given a set of packngo.IPAddressReservation and a set of tags, find
// the first reservation that has all of those tags
func ipReservationByAllTags(targetTags []string, ips []packngo.IPAddressReservation) *packngo.IPAddressReservation {
	ret := ipReservationsByAllTags(targetTags, ips)
	if len(ret) > 0 {
		return ret[0]
	}
	return nil
}

// ipReservationsByAllTags given a set of packngo.IPAddressReservation and a set of tags, find
// all of the reservations that have all of those tags
func ipReservationsByAllTags(targetTags []string, ips []packngo.IPAddressReservation) []*packngo.IPAddressReservation {
	// cycle through the IPs, looking for one that matches ours
	ret := []*packngo.IPAddressReservation{}
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
		ret = append(ret, &ips[i])
	}
	return ret
}

// ipReservationByAnyTags given a set of packngo.IPAddressReservation and a set of tags, find
// the first reservation that has any of those tags
func ipReservationByAnyTags(targetTags []string, ips []packngo.IPAddressReservation) *packngo.IPAddressReservation {
	ret := ipReservationsByAnyTags(targetTags, ips)
	if len(ret) > 0 {
		return ret[0]
	}
	return nil
}

// ipReservationsByAnyTags given a set of packngo.IPAddressReservation and a set of tags, find
// the reservations that have any of those tags
func ipReservationsByAnyTags(targetTags []string, ips []packngo.IPAddressReservation) []*packngo.IPAddressReservation {
	ret := []*packngo.IPAddressReservation{}
	tagMatches := map[string]bool{}
	for _, t := range targetTags {
		tagMatches[t] = true
	}
	// cycle through the IPs, looking for one or more that match
	for i, ip := range ips {
		var found bool
		for _, tag := range ip.Tags {
			if _, ok := tagMatches[tag]; ok {
				found = true
				break
			}
		}
		if found {
			ret = append(ret, &ips[i])
		}
	}
	// if we made it here, nothing matched
	return ret
}
