package metal

import (
	"testing"

	metal "github.com/equinix/equinix-sdk-go/services/metalv1"
)

func TestIPReservationByAllTags(t *testing.T) {
	ips := &metal.IPReservationList{
		IpAddresses: []metal.IPReservationListIpAddressesInner{
			{IPReservation: &metal.IPReservation{Tags: []string{"a", "b"}}},
			{IPReservation: &metal.IPReservation{Tags: []string{"c", "d"}}},
			{IPReservation: &metal.IPReservation{Tags: []string{"a", "d"}}},
			{IPReservation: &metal.IPReservation{Tags: []string{"b", "c"}}},
			{IPReservation: &metal.IPReservation{Tags: []string{"b", "q"}}},
		},
	}
	tests := []struct {
		tags  []string
		match int
	}{
		{[]string{"a"}, 0},
		{[]string{"a", "b"}, 0},
		{[]string{"b"}, 0},
		{[]string{"c"}, 1},
		{[]string{"d"}, 1},
		{[]string{"q"}, 4},
		{[]string{"q", "n"}, -1},
	}

	for i, tt := range tests {
		matched := ipReservationByAllTags(tt.tags, ips)
		switch {
		case matched == nil && tt.match >= 0:
			t.Errorf("%d: found no match but expected index %d", i, tt.match)
		case matched != nil && tt.match < 0:
			t.Errorf("%d: found a match but expected none", i)
		case matched == nil && tt.match < 0:
			// this is good
		case matched != ips.GetIpAddresses()[tt.match].IPReservation:
			t.Errorf("%d: match did not find index %d", i, tt.match)
		}
	}
}
