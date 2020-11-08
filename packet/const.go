package packet

type UpdateMode int

const (
	ModeAdd UpdateMode = iota
	ModeRemove
	ModeSync
)
