package metallb

type Resource struct {
	Namespace string
	Name      string
}
type Resources []Resource

// Len is part of sort.Interface.
func (r Resources) Len() int {
	return len(r)
}

// Swap is part of sort.Interface.
func (r Resources) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Less is part of sort.Interface
func (r Resources) Less(i, j int) bool {
	if r[i].Namespace < r[j].Namespace {
		return true
	}
	if r[i].Name < r[j].Name {
		return true
	}
	return false
}
