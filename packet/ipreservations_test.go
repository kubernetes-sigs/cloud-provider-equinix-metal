package packet

import (
	"testing"

	"github.com/packethost/packngo"
)

func TestIPReservationByTags(t *testing.T) {
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
		{[]string{"b"}, 0},
		{[]string{"c"}, 1},
		{[]string{"d"}, 1},
		{[]string{"q"}, 4},
	}

	for i, tt := range tests {
		matched := ipReservationByTags(tt.tags, ips)
		if matched != &ips[tt.match] {
			t.Errorf("%d: match did not find index %d", i, tt.match)
		}
	}

}
