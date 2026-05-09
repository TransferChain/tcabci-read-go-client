package tcabcireadgoclient

type Mode int

const (
	Subscription Mode = iota
	Listen
	Push
)
