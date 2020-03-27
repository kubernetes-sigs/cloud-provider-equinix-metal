package metallb

/*
 contains additional funcs, not in original metallb packages
*/

import (
	"fmt"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

func ParseConfig(bs []byte) (*ConfigFile, error) {
	var raw ConfigFile
	if err := yaml.UnmarshalStrict(bs, &raw); err != nil {
		return nil, fmt.Errorf("could not parse config: %s", err)
	}

	return &raw, nil
}

func (cfg *ConfigFile) Bytes() ([]byte, error) {
	return yaml.Marshal(cfg)
}

// AddPeer adds a peer. If a matching peer already exists, do not change anything
func (cfg *ConfigFile) AddPeer(add *Peer) {
	// ignore empty peer; nothing to add
	if add == nil {
		return
	}
	var found bool

	// go through the peers and see if we have one that matches
	// definition of a match is:
	// - MyASN matches
	// - ASN matches
	// - Addr matches
	// - NodeSelectors all match (but order is ignored)
	for _, peer := range cfg.Peers {
		if !peer.Equal(add) {
			continue
		}
		// they were equal, so we found a matcher
		found = true
	}
	if !found {
		cfg.Peers = append(cfg.Peers, *add)
	}
}

// RemovePeer remove a peer. If the matching peer does not exist, do not change anything
func (cfg *ConfigFile) RemovePeer(remove *Peer) {
	if remove == nil {
		return
	}
	// go through the peers and see if we have a match
	peers := make([]Peer, 0)
	// remove that one, keep all others
	for _, peer := range cfg.Peers {
		if !peer.Equal(remove) {
			peers = append(peers, peer)
		}
	}
	cfg.Peers = peers
}

// RemovePeerBySelector remove a peer by selector. If the matching peer does not exist, do not change anything
func (cfg *ConfigFile) RemovePeerBySelector(remove *NodeSelector) {
	if remove == nil {
		return
	}
	// go through the peers and see if we have a match
	peers := make([]Peer, 0)
	for _, peer := range cfg.Peers {
		if !peer.MatchSelector(remove) {
			peers = append(peers, peer)
		}
	}
	cfg.Peers = peers
}

// AddAddressPool adds an address pool. If a matching pool already exists, do not change anything
func (cfg *ConfigFile) AddAddressPool(add *AddressPool) {
	// ignore empty peer; nothing to add
	if add == nil {
		return
	}
	var found bool

	// go through the pools and see if we have one that matches
	for _, pool := range cfg.Pools {
		if !pool.Equal(add) {
			continue
		}
		// they were equal, so we found a matcher
		found = true
	}
	if !found {
		cfg.Pools = append(cfg.Pools, *add)
	}
}

// RemoveAddressPool remove a pool. If the matching pool does not exist, do not change anything
func (cfg *ConfigFile) RemoveAddressPool(remove *AddressPool) {
	if remove == nil {
		return
	}
	// go through the pools and see if we have a match
	pools := make([]AddressPool, 0)
	// remove that one, keep all others
	for _, pool := range cfg.Pools {
		if !pool.Equal(remove) {
			pools = append(pools, pool)
		}
	}
	cfg.Pools = pools
}

// RemoveAddressPooByAddress remove a pool by an address alone. If the matching pool does not exist, do not change anything
func (cfg *ConfigFile) RemoveAddressPoolByAddress(addr string) {
	if addr == "" {
		return
	}
	// go through the pools and see if we have a match
	pools := make([]AddressPool, 0)
	// go through the pools and see if we have one with our hostname
	for _, pool := range cfg.Pools {
		var found bool
		for _, ipaddr := range pool.Addresses {
			if addr == ipaddr {
				found = true
			}
		}
		if !found {
			pools = append(pools, pool)
		}
	}
	cfg.Pools = pools
}

type NodeSelectors []NodeSelector

func (n NodeSelectors) Len() int {
	return len(n)
}

func (n NodeSelectors) Less(i, j int) bool {
	if n[i].MatchLabels == nil && n[j].MatchLabels != nil {
		return true
	}
	if n[i].MatchLabels != nil && n[j].MatchLabels == nil {
		return false
	}
	// sort first by MatchLabels, then by MatchExpressions
	if len(n[i].MatchLabels) != len(n[j].MatchLabels) {
		return len(n[i].MatchLabels) < len(n[j].MatchLabels)
	}
	// same length, so go through them, but sort first
	ikeys := []string{}
	jkeys := []string{}
	for k := range n[i].MatchLabels {
		ikeys = append(ikeys, k)
	}
	for k := range n[j].MatchLabels {
		jkeys = append(jkeys, k)
	}
	sort.Strings(ikeys)
	sort.Strings(jkeys)
	for ii, k := range ikeys {
		if k != jkeys[ii] {
			return k < jkeys[ii]
		}
		if n[i].MatchLabels[k] != n[j].MatchLabels[k] {
			return n[i].MatchLabels[k] < n[j].MatchLabels[k]
		}
	}

	// MatchLabels are identical
	if n[i].MatchExpressions == nil && n[j].MatchExpressions != nil {
		return true
	}
	if n[i].MatchExpressions != nil && n[j].MatchExpressions == nil {
		return false
	}
	if len(n[i].MatchExpressions) != len(n[j].MatchExpressions) {
		return len(n[i].MatchExpressions) < len(n[j].MatchExpressions)
	}
	// same length, so compare
	var ime, jme SelectorRequirementsSlice = n[i].MatchExpressions[:], n[j].MatchExpressions[:]
	sort.Sort(ime)
	sort.Sort(jme)
	for ii, v := range ime {
		compare := v.Compare(&jme[ii])
		if compare < 0 {
			return true
		}
	}
	return false
}

func (n NodeSelectors) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n NodeSelectors) Equal(o NodeSelectors) bool {
	// not matched if the node selectors are of the wrong length
	if len(n) != len(o) {
		return false
	}

	// copy so that our sort does not affect the original
	n1 := n[:]
	o1 := o[:]
	sort.Sort(n1)
	sort.Sort(o1)
	for i, p := range n1 {
		if !p.Equal(&o1[i]) {
			return false
		}
	}
	return true
}

func (s *SelectorRequirements) Compare(o *SelectorRequirements) int {
	if s.Key != o.Key {
		return strings.Compare(s.Key, o.Key)
	}
	if s.Operator != o.Operator {
		return strings.Compare(s.Operator, o.Operator)
	}
	switch {
	case s.Values == nil && o.Values == nil:
		return 0
	case s.Values == nil && o.Values != nil:
		return -1
	case s.Values != nil && o.Values == nil:
		return 1
	case len(s.Values) < len(o.Values):
		return -1
	case len(s.Values) > len(o.Values):
		return 1
	default:
		// we sort before comparing, since the order is non-binding
		sValues, oValues := s.Values[:], o.Values[:]
		sort.Strings(sValues)
		sort.Strings(oValues)
		for i, v := range sValues {
			if v != oValues[i] {
				return strings.Compare(v, oValues[i])
			}
		}
	}

	// they are identical
	return 0
}

func (s *SelectorRequirements) Equal(o *SelectorRequirements) bool {
	return s.Compare(o) == 0
}

type SelectorRequirementsSlice []SelectorRequirements

func (s SelectorRequirementsSlice) Len() int {
	return len(s)
}

func (s SelectorRequirementsSlice) Less(i, j int) bool {
	// sort by key, then by operator, then by len(values), then by sorted values
	if s[i].Key != s[j].Key {
		return s[i].Key < s[j].Key
	}
	if s[i].Operator != s[j].Operator {
		return s[i].Operator < s[j].Operator
	}
	if len(s[i].Values) != len(s[j].Values) {
		return len(s[i].Values) < len(s[j].Values)
	}
	// just sort and compare lexicographically
	iValues, jValues := s[i].Values[:], s[j].Values[:]
	sort.Strings(iValues)
	sort.Strings(jValues)
	for ii, v := range iValues {
		if v != jValues[ii] {
			return v < jValues[ii]
		}
	}
	// if we got here, the two were identical
	return false
}

func (s SelectorRequirementsSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SelectorRequirementsSlice) Equal(o SelectorRequirementsSlice) bool {
	// not matched if the slices are of the wrong length
	if len(s) != len(o) {
		return false
	}

	// copy so that our sort does not affect the original
	s1 := s[:]
	o1 := o[:]
	sort.Sort(s1)
	sort.Sort(o1)
	for i, p := range s1 {
		if !p.Equal(&o1[i]) {
			return false
		}
	}
	return true
}

func (p *Peer) Equal(o *Peer) bool {
	if o == nil {
		return false
	}
	// not matched if any field is mismatched
	if p.MyASN != o.MyASN || p.ASN != o.ASN || p.Addr != o.Addr || p.Port != o.Port || p.HoldTime != o.HoldTime ||
		p.Password != o.Password || p.RouterID != o.RouterID {
		return false
	}

	var pns, ons NodeSelectors = p.NodeSelectors, o.NodeSelectors
	return pns.Equal(ons)
}

func (p *Peer) Duplicate() Peer {
	nodeSelectors := []NodeSelector{}
	for _, ns := range p.NodeSelectors {
		nodeSelectors = append(nodeSelectors, ns.Duplicate())
	}

	o := Peer{
		MyASN:         p.MyASN,
		ASN:           p.ASN,
		Addr:          p.Addr,
		Port:          p.Port,
		HoldTime:      p.HoldTime,
		Password:      p.Password,
		RouterID:      p.RouterID,
		NodeSelectors: nodeSelectors,
	}
	return o
}

// MatchSelector report if this peer matches a given selector
func (p *Peer) MatchSelector(s *NodeSelector) bool {
	for _, selector := range p.NodeSelectors {
		if selector.Equal(s) {
			return true
		}
	}
	return false
}

func (ns *NodeSelector) Equal(o *NodeSelector) bool {
	if o == nil {
		return false
	}
	if len(ns.MatchLabels) != len(o.MatchLabels) || len(ns.MatchExpressions) != len(o.MatchExpressions) {
		return false
	}
	for k, v := range ns.MatchLabels {
		if o.MatchLabels[k] != v {
			return false
		}
	}

	var pns, ons SelectorRequirementsSlice = ns.MatchExpressions, o.MatchExpressions
	return pns.Equal(ons)
}

func (ns *NodeSelector) Duplicate() NodeSelector {
	matchLabels := map[string]string{}
	for k, v := range ns.MatchLabels {
		matchLabels[k] = v
	}
	matchExpressions := []SelectorRequirements{}
	for _, s := range ns.MatchExpressions {
		s2 := SelectorRequirements{
			Key:      s.Key,
			Operator: s.Operator,
			Values:   s.Values[:],
		}
		matchExpressions = append(matchExpressions, s2)
	}

	o := NodeSelector{
		MatchLabels:      matchLabels,
		MatchExpressions: matchExpressions,
	}
	return o
}

// Equal determine if two AddressPools are equal. Definition of a match is:
// - Protocol matches
// - Name matches
// - AvoidBuggyIPs matches
// - AutoAssign matches
// - Addresses match (order is ignored)
// - BGPAdvertisements all match (order is ignored)
func (a *AddressPool) Equal(o *AddressPool) bool {
	// not matched if any field is mismatched
	if o == nil || a.Protocol != o.Protocol || a.Name != o.Name ||
		a.AvoidBuggyIPs != o.AvoidBuggyIPs || *a.AutoAssign != *o.AutoAssign {
		return false
	}

	// compare addresses
	if len(a.Addresses) != len(o.Addresses) {
		return false
	}
	// copy them so we do not mess up the original order
	aaddrs, oaddrs := a.Addresses[:], o.Addresses[:]
	sort.Strings(aaddrs)
	sort.Strings(oaddrs)
	for i, v := range aaddrs {
		if v != oaddrs[i] {
			return false
		}
	}

	// compare bgp advertisements
	if len(a.BGPAdvertisements) != len(o.BGPAdvertisements) {
		return false
	}
	// copy them so we do not mess up the original order
	var abgp, obgp BgpAdvertisements = a.BGPAdvertisements[:], o.BGPAdvertisements[:]
	sort.Sort(abgp)
	sort.Sort(obgp)

	for i, v := range abgp {
		if !v.Equal(&obgp[i]) {
			return false
		}
	}

	return true
}

func (a *AddressPool) Duplicate() AddressPool {
	// copy the value referenced by the AutoAssign bool pointer
	aa := *a.AutoAssign
	baa := aa
	// deep copy the BGP Advertisements
	bgpads := []BgpAdvertisement{}
	for _, bgp := range a.BGPAdvertisements {
		bgpads = append(bgpads, bgp.Duplicate())
	}
	b := AddressPool{
		Protocol:          a.Protocol,
		Name:              a.Name,
		Addresses:         a.Addresses[:],
		AvoidBuggyIPs:     a.AvoidBuggyIPs,
		AutoAssign:        &baa,
		BGPAdvertisements: bgpads,
	}
	return b
}

func (b *BgpAdvertisement) Equal(o *BgpAdvertisement) bool {
	if o == nil || *b.AggregationLength != *o.AggregationLength || *b.LocalPref != *o.LocalPref {
		return false
	}
	// copy them so we do not mess up the original order
	acomms, ocomms := b.Communities[:], o.Communities[:]
	sort.Strings(acomms)
	sort.Strings(ocomms)
	for i, v := range acomms {
		if v != ocomms[i] {
			return false
		}
	}

	return true
}

func (b *BgpAdvertisement) Duplicate() BgpAdvertisement {
	length := *b.AggregationLength
	pref := *b.LocalPref
	o := BgpAdvertisement{
		AggregationLength: &length,
		LocalPref:         &pref,
		Communities:       b.Communities[:],
	}
	return o
}

type BgpAdvertisements []BgpAdvertisement

func (b BgpAdvertisements) Len() int {
	return len(b)
}
func (b BgpAdvertisements) Less(i, j int) bool {
	if *b[i].AggregationLength < *b[j].AggregationLength || *b[i].LocalPref < *b[j].LocalPref {
		return true
	}
	// compare the strings
	if len(b[i].Communities) < len(b[j].Communities) {
		return true
	}
	icomms, jcomms := b[i].Communities, b[j].Communities
	sort.Strings(icomms)
	sort.Strings(jcomms)
	for ii, v := range icomms {
		if v < jcomms[ii] {
			return true
		}
	}

	return false
}
func (b BgpAdvertisements) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

/*
type BgpAdvertisement struct {
	AggregationLength *int `yaml:"aggregation-length"`
	LocalPref         *uint32
	Communities       []string
}

*/
