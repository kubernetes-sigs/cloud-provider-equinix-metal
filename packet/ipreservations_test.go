package packet

import (
	"testing"

	"github.com/packethost/packngo"
)

func TestIPReservationByAllTags(t *testing.T) {
	ips := []packngo.IPAddressReservation{
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"a", "b"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"c", "d"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"a", "d"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"b", "c"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"b", "q"}}},
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
		case matched != &ips[tt.match]:
			t.Errorf("%d: match did not find index %d", i, tt.match)
		}
	}
}

func TestIPReservationByAnyTags(t *testing.T) {
	ips := []packngo.IPAddressReservation{
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"a", "b"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"c", "d"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"a", "d"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"b", "c"}}},
		{IpAddressCommon: packngo.IpAddressCommon{Tags: []string{"b", "q"}}},
	}
	tests := []struct {
		tags  []string
		match int
	}{
		{[]string{"a", "c"}, 0},
		{[]string{"b", "q"}, 0},
		{[]string{"c", "n"}, 1},
		{[]string{"d", "l"}, 1},
		{[]string{"q", "g"}, 4},
		{[]string{"r", "g"}, -1},
	}

	for i, tt := range tests {
		matched := ipReservationByAnyTags(tt.tags, ips)
		switch {
		case matched == nil && tt.match >= 0:
			t.Errorf("%d: found no match but expected index %d", i, tt.match)
		case matched != nil && tt.match < 0:
			t.Errorf("%d: found a match but expected none", i)
		case matched == nil && tt.match < 0:
			// this is good
		case matched != &ips[tt.match]:
			t.Errorf("%d: match did not find index %d", i, tt.match)
		}
	}
}
