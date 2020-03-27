package metallb

import "math/rand"

func genRandomString(l int) string {
	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, l)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func genBGPAdvertisement() BgpAdvertisement {
	length := rand.Intn(100)
	pref := uint32(rand.Intn(100))

	return BgpAdvertisement{
		AggregationLength: &length,
		LocalPref:         &pref,
		Communities: []string{
			genRandomString(4),
			genRandomString(4),
		},
	}
}

func genPool() AddressPool {
	bgpadd := []BgpAdvertisement{
		genBGPAdvertisement(),
		genBGPAdvertisement(),
	}
	falseval := false
	pool := AddressPool{
		Protocol:          "bgp",
		Name:              "goodpool",
		Addresses:         []string{genRandomString(3), genRandomString(3)},
		AvoidBuggyIPs:     true,
		AutoAssign:        &falseval,
		BGPAdvertisements: bgpadd,
	}
	return pool
}

func genNodeSelector() NodeSelector {
	labels := map[string]string{
		genRandomString(4): genRandomString(4),
		genRandomString(4): genRandomString(4),
	}
	ns := NodeSelector{
		MatchLabels: labels,
		MatchExpressions: []SelectorRequirements{
			genSelectorRequirements(),
			genSelectorRequirements(),
			genSelectorRequirements(),
		},
	}
	return ns
}
func genSelectorRequirements() SelectorRequirements {
	return SelectorRequirements{
		Key:      genRandomString(4),
		Operator: genRandomString(4),
		Values: []string{
			genRandomString(5),
			genRandomString(5),
			genRandomString(5),
		},
	}
}

func genPeer() Peer {
	return Peer{
		MyASN:    uint32(rand.Intn(75000)),
		ASN:      uint32(rand.Intn(75000)),
		Addr:     genRandomString(10),
		Port:     uint16(rand.Intn(10000)),
		HoldTime: genRandomString(4),
		Password: genRandomString(10),
		RouterID: genRandomString(4),
		NodeSelectors: []NodeSelector{
			genNodeSelector(),
			genNodeSelector(),
		},
	}
}
