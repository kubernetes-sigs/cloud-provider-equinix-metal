package metallb

import (
	"fmt"
	"strings"
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

func TestConfigFileAddPeerByService(t *testing.T) {
	peers := []Peer{
		genPeer(Resource{"default", "a"}),
		genPeer(Resource{"default", "a"}),
	}
	cfg := ConfigFile{
		Peers: peers,
	}

	tests := []struct {
		peer                Peer
		total               int
		index               int
		addServiceNamespace string
		addServicename      string
		expectedServices    []string
		message             string
	}{
		{peers[0], len(peers), 0, "default", "a", []string{"a"}, "add existing peer with existing service"},
		{peers[1], len(peers), 1, "default", "b", []string{"a", "b"}, "add existing peer with new service"},
		{genPeer(), len(peers) + 1, len(peers), "default", "c", []string{"c"}, "add new peer"},
	}

	for i, tt := range tests {
		// get a clean set of peers
		cfg.Peers = peers[:]
		cfg.AddPeerByService(&tt.peer, tt.addServiceNamespace, tt.addServicename)
		// make sure the number of peers is as expected
		if len(cfg.Peers) != tt.total {
			t.Fatalf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Peers), tt.total, tt.message)
		}
		// make sure the particular peer has the right services annotated
		p := cfg.Peers[tt.index]

		var (
			svcs  []string
			found bool
		)
		for _, ns := range p.NodeSelectors {
			var namespace, name string
			for k, v := range ns.MatchLabels {
				switch k {
				case serviceNamespaceKey:
					namespace = v
				case serviceNameKey:
					name = v
				}
				if namespace == tt.addServiceNamespace && name == tt.addServicename {
					found = true
				}
				svcs = append(svcs, fmt.Sprintf("%s/%s", namespace, name))
			}
		}
		if !found {
			t.Fatalf("%d: could not find node selector with the right services label: %s", i, serviceNameKey)
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

func TestConfigFileRemovePeersByService(t *testing.T) {
	services := [][]Resource{
		{Resource{"default", "a"}},
		{Resource{"default", "a"}, Resource{"default", "b"}},
		{Resource{"default", "c"}},
	}

	tests := []struct {
		left         [][]Resource
		svcNamespace string
		svcName      string
		message      string
	}{
		{services[0:2], "default", "c", "remove lone service from existing peer"},
		{services, "default", "b", "remove non-lone service from existent peer"},
		{services[1:3], "default", "a", "remove service from multiple peers"},
		{services[:], "default", "d", "remove service from non-existent peer"},
	}

	for i, tt := range tests {
		// get a clean set of peers
		var peers []Peer
		for _, list := range services {
			peers = append(peers, genPeer(list...))
		}
		cfg := ConfigFile{
			Peers: peers,
		}
		cfg.RemovePeersByService(tt.svcNamespace, tt.svcName)
		if len(cfg.Peers) != len(tt.left) {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Peers), len(tt.left), tt.message)
		}
		// make sure no peer has the removed service annotated
		for _, p := range cfg.Peers {
			var (
				svcs  []string
				found bool
			)
			for _, ns := range p.NodeSelectors {
				var namespace, name string
				for k, v := range ns.MatchLabels {
					switch k {
					case serviceNamespaceKey:
						namespace = v
					case serviceNameKey:
						name = v
					}
					if namespace == tt.svcNamespace && name == tt.svcName {
						found = true
					}
					svcs = append(svcs, fmt.Sprintf("%s/%s", namespace, name))
				}
			}
			if found {
				t.Errorf("%d: still has service '%s/%s' after removal, list: %s", i, tt.svcNamespace, tt.svcName, svcs)
			}
		}
	}
}

func TestConfigFileRemovePeersBySelector(t *testing.T) {
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
		cfg.RemovePeersBySelector(&tt.selector)
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
	modifiedPool := pools[0].Duplicate()
	modifiedPool.Name = "newName"
	joinedPool := pools[0].Duplicate()
	joinedPool.Name = strings.Join([]string{pools[0].Name, modifiedPool.Name}, nameJoiner)
	modifiedPools := []AddressPool{
		joinedPool,
		pools[1],
	}

	newPool := genPool()

	tests := []struct {
		pool     AddressPool
		changed  bool
		expected []AddressPool
		message  string
	}{
		{pools[0], false, pools, "add existing first pool with same name"},
		{pools[1], false, pools, "add existing second pool with same name"},
		{modifiedPool, true, modifiedPools, "add existing first pool with different name"},
		{newPool, true, append(pools, newPool), "new pool"},
	}

	for i, tt := range tests {
		cfg := ConfigFile{
			Pools: append([]AddressPool{}, pools...),
		}

		changed := cfg.AddAddressPool(&tt.pool)
		if changed != tt.changed {
			t.Errorf("%d: mismatched changed actual %v expected %v", i, changed, tt.changed)
		}
		if len(cfg.Pools) != len(tt.expected) {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Pools), len(tt.expected), tt.message)
		} else {
			for j, pool := range cfg.Pools {
				if !pool.Equal(&tt.expected[j]) {
					t.Errorf("%d: pool %d mismatched, actual %#v, expected %#v", i, j, pool, tt.expected[j])
				}
			}
		}
	}
}
func TestConfigFileRemoveAddressPool(t *testing.T) {
	pools := []AddressPool{
		genPool(),
		genPool(),
	}
	joinedPool := genPool()
	joinedPool.Name = "newName,oldName"
	pools = append(pools, joinedPool)
	unjoinedPoolOld := joinedPool.Duplicate()
	unjoinedPoolOld.Name = "oldName"
	unjoinedPoolNew := joinedPool.Duplicate()
	unjoinedPoolNew.Name = "newName"
	modifiedPools := pools[:]
	modifiedPools[2] = unjoinedPoolNew

	tests := []struct {
		pool     AddressPool
		expected []AddressPool
		message  string
	}{
		{pools[0], pools[1:], "remove first existing pool"},
		{pools[1], []AddressPool{pools[0], pools[2]}, "remove second existing pool"},
		{genPool(), pools, "no match"},
		{unjoinedPoolOld, modifiedPools, "remove single name of joined pool"},
	}

	for i, tt := range tests {
		cfg := ConfigFile{
			Pools: append([]AddressPool{}, pools...),
		}

		cfg.RemoveAddressPool(&tt.pool)
		if len(cfg.Pools) != len(tt.expected) {
			t.Errorf("%d: mismatch actual %d vs expected %d: %s", i, len(cfg.Pools), len(tt.expected), tt.message)
		} else {
			for j, pool := range cfg.Pools {
				if !pool.Equal(&tt.expected[j]) {
					t.Errorf("%d: pool %d mismatched, actual %#v, expected %#v", i, j, pool, tt.expected[j])
				}
			}
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
		"empty": {},
		"ad": {
			Key:      "a",
			Operator: "d",
			Values:   []string{},
		},
		"dd": {
			Key:      "d",
			Operator: "d",
			Values:   []string{},
		},
		"ap": {
			Key:      "a",
			Operator: "p",
			Values:   []string{},
		},
		"ap-values-empty": {
			Key:      "a",
			Operator: "p",
			Values:   []string{},
		},
		"ap-values-nil": {
			Key:      "a",
			Operator: "p",
			Values:   nil,
		},
		"ap-values-a": {
			Key:      "a",
			Operator: "p",
			Values:   []string{"a"},
		},
		"ap-values-b": {
			Key:      "a",
			Operator: "p",
			Values:   []string{"b"},
		},
		"ap-values-ab": {
			Key:      "a",
			Operator: "p",
			Values:   []string{"a", "b"},
		},
		"ap-values-ap": {
			Key:      "a",
			Operator: "p",
			Values:   []string{"a", "p"},
		},
		"ap-values-pa": {
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
	// basic set we use for our tests.
	/*
		type SelectorRequirements struct {
			Key      string   `yaml:"key"`
			Operator string   `yaml:"operator"`
			Values   []string `yaml:"values"`
		}
	*/
	sr := map[string]SelectorRequirements{
		"mykey-equal-abc": {
			Key:      "mykey",
			Operator: "equal",
			Values:   []string{"a", "b", "c"},
		},
		"yourkey-equal-abc": {
			Key:      "yourkey",
			Operator: "equal",
			Values:   []string{"a", "b", "c"},
		},
		"mykey-equal-def": {
			Key:      "mykey",
			Operator: "equal",
			Values:   []string{"d", "e", "f"},
		},
		"mykey-equal-ab": {
			Key:      "mykey",
			Operator: "equal",
			Values:   []string{"a", "b"},
		},
		"mykey-equal-de": {
			Key:      "mykey",
			Operator: "equal",
			Values:   []string{"d", "e"},
		},
		"yourkey-equal-def": {
			Key:      "yourkey",
			Operator: "equal",
			Values:   []string{"d", "e", "f"},
		},
		"yourkey-equal-de": {
			Key:      "yourkey",
			Operator: "equal",
			Values:   []string{"d", "e"},
		},
		"mykey-in-abc": {
			Key:      "mykey",
			Operator: "in",
			Values:   []string{"a", "b", "c"},
		},
		"mykey-in-ab": {
			Key:      "mykey",
			Operator: "in",
			Values:   []string{"a", "b"},
		},
		"yourkey-in-abc": {
			Key:      "yourkey",
			Operator: "in",
			Values:   []string{"a", "b", "c"},
		},
		"mykey-in-def": {
			Key:      "mykey",
			Operator: "in",
			Values:   []string{"d", "e", "f"},
		},
		"yourkey-in-def": {
			Key:      "yourkey",
			Operator: "in",
			Values:   []string{"d", "e", "f"},
		},
	}
	tests := []struct {
		left, right string
		less        bool
	}{
		{"mykey-equal-abc", "mykey-equal-def", true},
		{"mykey-equal-abc", "yourkey-equal-abc", true},
		{"mykey-equal-abc", "yourkey-equal-def", true},
		{"mykey-equal-abc", "mykey-equal-ab", false},
		{"mykey-equal-ab", "mykey-equal-abc", true},
		{"mykey-equal-abc", "mykey-equal-de", false},
		{"mykey-equal-de", "mykey-equal-abc", true},
		{"mykey-equal-de", "mykey-equal-ab", false},
		{"mykey-equal-ab", "mykey-equal-de", true},
		{"mykey-equal-abc", "yourkey-equal-de", true},
		{"mykey-equal-abc", "mykey-in-abc", true},
		{"mykey-in-abc", "mykey-equal-abc", false},
		{"mykey-in-ab", "mykey-equal-def", false},
		{"mykey-in-def", "mykey-in-abc", false},
		{"mykey-in-abc", "mykey-in-def", true},
		{"mykey-in-abc", "yourkey-in-def", true},
		{"mykey-in-abc", "yourkey-equal-abc", true},
		{"mykey-in-abc", "mykey-in-ab", false},
		{"mykey-in-abc", "mykey-in-abc", false},
	}
	for i, tt := range tests {
		if _, ok := sr[tt.left]; !ok {
			t.Fatalf("unknown entry: %s", tt.left)
		}
		if _, ok := sr[tt.right]; !ok {
			t.Fatalf("unknown entry: %s", tt.right)
		}
		var srs SelectorRequirementsSlice = []SelectorRequirements{
			sr[tt.left],
			sr[tt.right],
		}
		actual := srs.Less(0, 1)
		if actual != tt.less {
			t.Errorf("%d: left %s vs right %s gave %v instead of expected %v", i, tt.left, tt.right, actual, tt.less)
		}
	}
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
