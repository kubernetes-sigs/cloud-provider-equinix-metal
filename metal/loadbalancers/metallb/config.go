package metallb

// these are taken from https://github.com/metallb/metallb/blob/master/internal/config/config.go
// unfortunately, these are internal, so we cannot leverage them and need to copy. :-()

// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// configFile is the configuration as parsed out of the ConfigMap,
// without validation or useful high level types.
type ConfigFile struct {
	Peers          []Peer
	BGPCommunities map[string]string `yaml:"bgp-communities"`
	Pools          []AddressPool     `yaml:"address-pools"`
}

type Peer struct {
	MyASN         uint32         `yaml:"my-asn"`
	ASN           uint32         `yaml:"peer-asn"`
	Addr          string         `yaml:"peer-address"`
	Port          uint16         `yaml:"peer-port"`
	HoldTime      string         `yaml:"hold-time"`
	RouterID      string         `yaml:"router-id"`
	NodeSelectors []NodeSelector `yaml:"node-selectors"`
	Password      string         `yaml:"password"`
}

type NodeSelector struct {
	MatchLabels      map[string]string      `yaml:"match-labels"`
	MatchExpressions []SelectorRequirements `yaml:"match-expressions"`
}

type SelectorRequirements struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values"`
}

type AddressPool struct {
	Protocol          Proto
	Name              string
	Addresses         []string
	AvoidBuggyIPs     bool               `yaml:"avoid-buggy-ips"`
	AutoAssign        *bool              `yaml:"auto-assign"`
	BGPAdvertisements []BgpAdvertisement `yaml:"bgp-advertisements"`
}

type BgpAdvertisement struct {
	AggregationLength *int `yaml:"aggregation-length"`
	LocalPref         *uint32
	Communities       []string
}

// Proto holds the protocol we are speaking.
type Proto string
