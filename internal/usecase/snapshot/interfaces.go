package snapshot

type ServiceI interface {
	GetVersion() string
	Get() []byte
	Refresh()
}
