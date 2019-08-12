package metallb

import (
	"testing"
)

func TestConfigFileAddPeer(t *testing.T) {
	peers := []Peer{
		genPeer(),
		genPeer(),
	}
	cfg := ConfigFile{
		Peers: peers,
	}

	tests := []struct {
		peer    Peer
		total   int
		message string
	}{
		{peers[0], len(peers), "add existing peer"},
		{genPeer(), len(peers) + 1, "add new peer"},
	}

	for i, tt := range tests {
		// get a clean set of peers
		cfg.Peers = peers[:]
		cfg.AddPeer(&tt.peer)
		if len(cfg.Peers) != tt.total {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Peers), tt.total, tt.message)
		}
	}
}

func TestConfigFileRemovePeer(t *testing.T) {
	peers := []Peer{
		genPeer(),
		genPeer(),
	}
	cfg := ConfigFile{
		Peers: peers,
	}

	tests := []struct {
		peer    Peer
		total   int
		message string
	}{
		{peers[0], len(peers) - 1, "remove existing peer"},
		{genPeer(), len(peers), "remove non-existent peer"},
		{peers[0].Duplicate(), len(peers) - 1, "remove matching peer"},
	}

	for i, tt := range tests {
		// get a clean set of peers
		cfg.Peers = peers[:]
		cfg.RemovePeer(&tt.peer)
		if len(cfg.Peers) != tt.total {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Peers), tt.total, tt.message)
		}
	}
}

func TestConfigFileRemovePeerBySelector(t *testing.T) {
	peers := []Peer{
		genPeer(),
		genPeer(),
	}
	cfg := ConfigFile{
		Peers: peers,
	}

	tests := []struct {
		selector NodeSelector
		total    int
		message  string
	}{
		{peers[0].NodeSelectors[0], len(peers) - 1, "remove existing peer, first selector"},
		{peers[0].NodeSelectors[1], len(peers) - 1, "remove existing peer, second selector"},
		{genNodeSelector(), len(peers), "no match"},
	}

	for i, tt := range tests {
		// get a clean set of peers
		cfg.Peers = peers[:]
		cfg.RemovePeerBySelector(&tt.selector)
		if len(cfg.Peers) != tt.total {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Peers), tt.total, tt.message)
		}
	}
}

func TestConfigFileAddAddressPool(t *testing.T) {
	// NOTE: this leverages AddressPool.Equal(), which we test below,
	// so no need to test the various forms of equality, just that it does the
	// right thing assuming it is equal
	pools := []AddressPool{
		genPool(),
		genPool(),
	}
	cfg := ConfigFile{
		Pools: pools,
	}

	tests := []struct {
		pool    AddressPool
		total   int
		message string
	}{
		{pools[0], len(pools), "add existing first pool"},
		{pools[1], len(pools), "add existing second pool"},
		{genPool(), len(pools) + 1, "new pool"},
	}

	for i, tt := range tests {
		cfg.Pools = pools[:]
		cfg.AddAddressPool(&tt.pool)
		if len(cfg.Pools) != tt.total {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Pools), tt.total, tt.message)
		}
	}
}
func TestConfigFileRemoveAddressPool(t *testing.T) {
	pools := []AddressPool{
		genPool(),
		genPool(),
	}
	cfg := ConfigFile{
		Pools: pools,
	}

	tests := []struct {
		pool    AddressPool
		total   int
		message string
	}{
		{pools[0], len(pools) - 1, "remove first existing pool"},
		{pools[1], len(pools) - 1, "remove second existing pool"},
		{genPool(), len(pools), "no match"},
	}

	for i, tt := range tests {
		cfg.Pools = pools[:]
		cfg.RemoveAddressPool(&tt.pool)
		if len(cfg.Pools) != tt.total {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Pools), tt.total, tt.message)
		}
	}
}
func TestConfigFileRemoveAddressPoolByAddress(t *testing.T) {
	pools := []AddressPool{
		genPool(),
		genPool(),
	}
	cfg := ConfigFile{
		Pools: pools,
	}

	tests := []struct {
		addr    string
		total   int
		message string
	}{
		{pools[0].Addresses[0], len(pools) - 1, "remove existing first pool, first address"},
		{pools[0].Addresses[1], len(pools) - 1, "remove existing first pool, second address"},
		{pools[1].Addresses[0], len(pools) - 1, "remove existing second pool, first address"},
		{pools[1].Addresses[1], len(pools) - 1, "remove existing second pool, second address"},
		{genRandomString(5), len(pools), "no match"},
	}

	for i, tt := range tests {
		cfg.Pools = pools[:]
		cfg.RemoveAddressPoolByAddress(tt.addr)
		if len(cfg.Pools) != tt.total {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Pools), tt.total, tt.message)
		}
	}
}

func TestNodeSelectorsLen(t *testing.T) {
	sl := []NodeSelector{
		genNodeSelector(),
		genNodeSelector(),
		genNodeSelector(),
	}
	var n NodeSelectors = sl
	if n.Len() != len(sl) {
		t.Errorf("mismatched, actual %d vs expected %d", n.Len(), len(sl))
	}
}

func TestNodeSelectorsLess(t *testing.T) {
	// basic set we use for our tests.
	ns := map[string]NodeSelector{
		"labels-aq": {
			MatchLabels:      map[string]string{"a": "b", "q": "r"},
			MatchExpressions: []SelectorRequirements{},
		},
		"labels-a": {
			MatchLabels:      map[string]string{"a": "b"},
			MatchExpressions: []SelectorRequirements{},
		},
		"labels-ae": {
			MatchLabels:      map[string]string{"a": "b", "e": "f"},
			MatchExpressions: []SelectorRequirements{},
		},
		"labels-empty": {
			MatchLabels:      map[string]string{},
			MatchExpressions: []SelectorRequirements{},
		},
		"labels-nil": {
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
		"expr-nil": {
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
		"expr-empty": {
			MatchLabels:      nil,
			MatchExpressions: []SelectorRequirements{},
		},
		"expr-ad": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "d",
					Values:   []string{},
				},
			},
		},
		"expr-dd": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "d",
					Operator: "d",
					Values:   []string{},
				},
			},
		},
		"expr-ap": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{},
				},
			},
		},
		"expr-ap-values-empty": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{},
				},
			},
		},
		"expr-ap-values-nil": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   nil,
				},
			},
		},
		"expr-ap-values-a": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{"a"},
				},
			},
		},
		"expr-ap-values-b": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{"b"},
				},
			},
		},
		"expr-ap-values-ab": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{"a", "b"},
				},
			},
		},
		"expr-ap-values-ap": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{"a", "p"},
				},
			},
		},
		"expr-ap-values-pa": {
			MatchLabels: nil,
			MatchExpressions: []SelectorRequirements{
				{
					Key:      "a",
					Operator: "p",
					Values:   []string{"p", "a"},
				},
			},
		},
	}
	tests := []struct {
		left, right string
		less        bool
	}{
		{"labels-a", "labels-aq", true},
		{"labels-aq", "labels-a", false},
		{"labels-a", "labels-ae", true},
		{"labels-ae", "labels-a", false},
		{"labels-ae", "labels-aq", true},
		{"labels-aq", "labels-ae", false},
		{"labels-empty", "labels-a", true},
		{"labels-a", "labels-empty", false},
		{"labels-nil", "labels-a", true},
		{"labels-a", "labels-nil", false},
		{"labels-nil", "labels-empty", true},
		{"labels-empty", "labels-nil", false},
		{"expr-empty", "expr-nil", false},
		{"expr-nil", "expr-empty", true},
		{"expr-ad", "expr-empty", false},
		{"expr-empty", "expr-ad", true},
		{"expr-ad", "expr-nil", false},
		{"expr-nil", "expr-ad", true},
		{"expr-ad", "expr-dd", true},
		{"expr-dd", "expr-ad", false},
		{"expr-ad", "expr-ap", true},
		{"expr-ap", "expr-ad", false},
		{"expr-ap-values-empty", "expr-ap-values-nil", false},
		{"expr-ap-values-nil", "expr-ap-values-empty", true},
		{"expr-ap-values-empty", "expr-ap-values-a", true},
		{"expr-ap-values-a", "expr-ap-values-empty", false},
		{"expr-ap-values-nil", "expr-ap-values-a", true},
		{"expr-ap-values-a", "expr-ap-values-nil", false},
		{"expr-ap-values-b", "expr-ap-values-a", false},
		{"expr-ap-values-a", "expr-ap-values-b", true},
		{"expr-ap-values-ab", "expr-ap-values-a", false},
		{"expr-ap-values-a", "expr-ap-values-ab", true},
		// order is non-binding
		{"expr-ap-values-ap", "expr-ap-values-ab", false},
		{"expr-ap-values-ab", "expr-ap-values-ap", true},
		{"expr-ap-values-ap", "expr-ap-values-pa", false},
		{"expr-ap-values-pa", "expr-ap-values-ap", false},
	}
	for i, tt := range tests {
		if _, ok := ns[tt.left]; !ok {
			t.Fatalf("unknown entry: %s", tt.left)
		}
		if _, ok := ns[tt.right]; !ok {
			t.Fatalf("unknown entry: %s", tt.right)
		}
		var nss NodeSelectors = []NodeSelector{
			ns[tt.left],
			ns[tt.right],
		}
		actual := nss.Less(0, 1)
		if actual != tt.less {
			t.Errorf("%d: left %s vs right %s gave %v instead of expected %v", i, tt.left, tt.right, actual, tt.less)
		}
	}
}

func TestNodeSelectorsEqual(t *testing.T) {
	sl := []NodeSelector{
		genNodeSelector(),
		genNodeSelector(),
		genNodeSelector(),
	}
	var n NodeSelectors = sl

	// different lengths
	if n.Equal(NodeSelectors(sl[0:2])) {
		t.Error("mismatched lengths reports equal")
	}
	// same lengths, different contents
	if n.Equal(NodeSelectors([]NodeSelector{
		sl[0],
		sl[1],
		genNodeSelector(),
	})) {
		t.Error("third different reports equal")
	}

	// same content, different order
	if !n.Equal(NodeSelectors([]NodeSelector{
		sl[2],
		sl[0],
		sl[1],
	})) {
		t.Error("same contents with different order reports unequal")
	}
}

func TestSelectorRequirementsCompare(t *testing.T) {
	// basic set we use for our tests.
	ns := map[string]SelectorRequirements{
		"empty": SelectorRequirements{},
		"ad": SelectorRequirements{
			Key:      "a",
			Operator: "d",
			Values:   []string{},
		},
		"dd": SelectorRequirements{
			Key:      "d",
			Operator: "d",
			Values:   []string{},
		},
		"ap": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{},
		},
		"ap-values-empty": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{},
		},
		"ap-values-nil": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   nil,
		},
		"ap-values-a": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{"a"},
		},
		"ap-values-b": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{"b"},
		},
		"ap-values-ab": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{"a", "b"},
		},
		"ap-values-ap": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{"a", "p"},
		},
		"ap-values-pa": SelectorRequirements{
			Key:      "a",
			Operator: "p",
			Values:   []string{"p", "a"},
		},
	}
	tests := []struct {
		left, right string
		compare     int
	}{
		{"ad", "empty", 1},
		{"empty", "ad", -1},
		{"ad", "dd", -1},
		{"dd", "ad", 1},
		{"ad", "ap", -1},
		{"ap", "ad", 1},
		{"ap-values-empty", "ap-values-nil", 1},
		{"ap-values-nil", "ap-values-empty", -1},
		{"ap-values-empty", "ap-values-empty", 0},
		{"ap-values-empty", "ap-values-a", -1},
		{"ap-values-a", "ap-values-empty", 1},
		{"ap-values-nil", "ap-values-a", -1},
		{"ap-values-a", "ap-values-nil", 1},
		{"ap-values-b", "ap-values-a", 1},
		{"ap-values-a", "ap-values-b", -1},
		{"ap-values-ab", "ap-values-a", 1},
		{"ap-values-a", "ap-values-ab", -1},
		{"ap-values-ap", "ap-values-ab", 1},
		{"ap-values-ab", "ap-values-ap", -1},
		// order is non-binding
		{"ap-values-ap", "ap-values-pa", 0},
		{"ap-values-pa", "ap-values-ap", 0},
	}
	for i, tt := range tests {
		var (
			left, right SelectorRequirements
			ok          bool
		)

		if left, ok = ns[tt.left]; !ok {
			t.Fatalf("unknown entry: %s", tt.left)
		}
		if right, ok = ns[tt.right]; !ok {
			t.Fatalf("unknown entry: %s", tt.right)
		}
		actual := left.Compare(&right)
		if actual != tt.compare {
			t.Errorf("%d: left %s vs right %s gave %v instead of expected %v", i, tt.left, tt.right, actual, tt.compare)
		}
	}

}

// we do not test SelectorRequirements.Equal, as it is just .Compare() == 0

func TestSelectorRequirementsSliceLen(t *testing.T) {
	sl := []SelectorRequirements{
		genSelectorRequirements(),
		genSelectorRequirements(),
		genSelectorRequirements(),
	}
	var s SelectorRequirementsSlice = sl
	if s.Len() != len(sl) {
		t.Errorf("mismatched, actual %d vs expected %d", s.Len(), len(sl))
	}
}

func TestSelectorRequirementsSliceLess(t *testing.T) {

}

func TestSelectorRequirementsSliceEqual(t *testing.T) {
	sl := []SelectorRequirements{
		genSelectorRequirements(),
		genSelectorRequirements(),
		genSelectorRequirements(),
	}
	var s SelectorRequirementsSlice = sl

	// different lengths
	if s.Equal(SelectorRequirementsSlice(sl[0:2])) {
		t.Error("mismatched lengths reports equal")
	}
	// same lengths, different contents
	if s.Equal(SelectorRequirementsSlice([]SelectorRequirements{
		sl[0],
		sl[1],
		genSelectorRequirements(),
	})) {
		t.Error("third different reports equal")
	}

	// same content, different order
	if !s.Equal(SelectorRequirementsSlice([]SelectorRequirements{
		sl[2],
		sl[0],
		sl[1],
	})) {
		t.Error("same contents with different order reports unequal")
	}
}
func TestPeerEqual(t *testing.T) {
	base := genPeer()

	// nil
	if base.Equal(nil) {
		t.Error("nil equal")
	}

	var other Peer

	other = base.Duplicate()
	if !base.Equal(&other) {
		t.Error("identical is not equal")
	}

	other = base.Duplicate()
	if !base.Equal(&other) {
		t.Error("identical is not equal")
	}

	other = base.Duplicate()
	other.MyASN++
	if base.Equal(&other) {
		t.Error("MyASN different: equal")
	}

	other = base.Duplicate()
	other.ASN++
	if base.Equal(&other) {
		t.Error("ASN different: equal")
	}

	other = base.Duplicate()
	other.Port++
	if base.Equal(&other) {
		t.Error("Port different: equal")
	}

	other = base.Duplicate()
	other.Addr = genRandomString(5)
	if base.Equal(&other) {
		t.Error("Addr different: equal")
	}

	other = base.Duplicate()
	other.HoldTime = genRandomString(5)
	if base.Equal(&other) {
		t.Error("HoldTime different: equal")
	}

	other = base.Duplicate()
	other.Password = genRandomString(5)
	if base.Equal(&other) {
		t.Error("Password different: equal")
	}

	other = base.Duplicate()
	other.RouterID = genRandomString(5)
	if base.Equal(&other) {
		t.Error("RouterID different: equal")
	}

	// NodeSelectors
	other = base.Duplicate()
	other.NodeSelectors = nil
	if base.Equal(&other) {
		t.Error("nil NodeSelectors: equal")
	}

	other = base.Duplicate()
	other.NodeSelectors = []NodeSelector{}
	if base.Equal(&other) {
		t.Error("empty NodeSelectors: equal")
	}

	other = base.Duplicate()
	other.NodeSelectors = base.NodeSelectors[0:1]
	if base.Equal(&other) {
		t.Error("first NodeSelectors: equal")
	}

	other = base.Duplicate()
	other.NodeSelectors = base.NodeSelectors[1:2]
	if base.Equal(&other) {
		t.Error("second NodeSelectors: equal")
	}

	other = base.Duplicate()
	other.NodeSelectors = []NodeSelector{
		base.NodeSelectors[1],
		base.NodeSelectors[0],
	}
	// order is irrelevant
	if !base.Equal(&other) {
		t.Error("out-of-order NodeSelectors: not equal")
	}
}
func TestPeerMatchSelector(t *testing.T) {

}
func TestNodeSelectorEqual(t *testing.T) {
	base := genNodeSelector()

	// nil
	if base.Equal(nil) {
		t.Error("nil equal")
	}

	var other NodeSelector
	other = base.Duplicate()
	if !base.Equal(&other) {
		t.Error("identical are not equal")
	}

	// matchLabels different
	other = base.Duplicate()
	other.MatchLabels = nil
	if base.Equal(&other) {
		t.Error("nil MatchLabels equal")
	}

	other = base.Duplicate()
	other.MatchLabels = map[string]string{}
	if base.Equal(&other) {
		t.Error("empty MatchLabels equal")
	}

	other = base.Duplicate()
	other.MatchLabels = map[string]string{
		genRandomString(4): genRandomString(4),
		genRandomString(4): genRandomString(4),
	}
	if base.Equal(&other) {
		t.Error("different MatchLabels equal")
	}

	// matchExpressions different
	other = base.Duplicate()
	other.MatchExpressions = nil
	if base.Equal(&other) {
		t.Error("nil MatchExpressions equal")
	}

	other = base.Duplicate()
	other.MatchExpressions = []SelectorRequirements{}
	if base.Equal(&other) {
		t.Error("empty MatchExpressions equal")
	}

	other = base.Duplicate()
	other.MatchExpressions = []SelectorRequirements{
		genSelectorRequirements(),
		genSelectorRequirements(),
		genSelectorRequirements(),
	}
	if base.Equal(&other) {
		t.Error("different MatchExpressions equal")
	}
}

func TestAddressPoolEqual(t *testing.T) {
	// base pool
	pool := genPool()

	// nil
	if pool.Equal(nil) {
		t.Error("nil equal")
	}

	var other AddressPool
	// identical values
	other = pool.Duplicate()
	if !pool.Equal(&other) {
		t.Error("identicals not equal")
	}

	// check individual values
	other = pool.Duplicate()
	other.Protocol = "other"
	if pool.Equal(&other) {
		t.Error("different protocols equal")
	}

	other = pool.Duplicate()
	other.Name = "badpool"
	if pool.Equal(&other) {
		t.Error("different names equal")
	}

	other = pool.Duplicate()
	other.AvoidBuggyIPs = !other.AvoidBuggyIPs
	if pool.Equal(&other) {
		t.Error("different AvoidBuggyIPs equal")
	}

	other = pool.Duplicate()
	aa := other.AutoAssign
	notaa := !(*aa)
	other.AutoAssign = &notaa
	if pool.Equal(&other) {
		t.Error("different AutoAssign equal")
	}

	// addresses
	other = pool.Duplicate()
	other.Addresses = nil
	if pool.Equal(&other) {
		t.Error("nil addresses equal")
	}

	other = pool.Duplicate()
	other.Addresses = []string{}
	if pool.Equal(&other) {
		t.Error("empty addresses equal")
	}

	other = pool.Duplicate()
	other.Addresses = other.Addresses[0:1]
	if pool.Equal(&other) {
		t.Error("first address only equal")
	}

	other = pool.Duplicate()
	other.Addresses = other.Addresses[1:2]
	if pool.Equal(&other) {
		t.Error("second address only equal")
	}

	other = pool.Duplicate()
	other.Addresses = []string{"mickey", "minnie"}
	if pool.Equal(&other) {
		t.Error("different addresses equal")
	}

	// BGPAdvertisements
	other = pool.Duplicate()
	other.BGPAdvertisements = nil
	if pool.Equal(&other) {
		t.Error("nil bgpadvertisements equal")
	}

	other = pool.Duplicate()
	other.BGPAdvertisements = []BgpAdvertisement{}
	if pool.Equal(&other) {
		t.Error("empty bgpadvertisements equal")
	}

	other = pool.Duplicate()
	other.BGPAdvertisements = []BgpAdvertisement{}
	if pool.Equal(&other) {
		t.Error("empty bgpadvertisements equal")
	}

	other = pool.Duplicate()
	other.BGPAdvertisements = []BgpAdvertisement{
		genBGPAdvertisement(),
	}
	if pool.Equal(&other) {
		t.Error("single bgpadvertisements equal")
	}

	other = pool.Duplicate()
	other.BGPAdvertisements = []BgpAdvertisement{
		genBGPAdvertisement(),
		genBGPAdvertisement(),
	}
	if pool.Equal(&other) {
		t.Error("two different bgpadvertisements equal")
	}

	other = pool.Duplicate()
	other.BGPAdvertisements = pool.BGPAdvertisements[0:1]
	if pool.Equal(&other) {
		t.Error("first of existing bgpadvertisements equal")
	}

	other = pool.Duplicate()
	other.BGPAdvertisements = pool.BGPAdvertisements[1:2]
	if pool.Equal(&other) {
		t.Error("second of existing bgpadvertisements equal")
	}
}
