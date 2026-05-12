package snapshot

type Usecase struct {
	svc ServiceI
}

func New(svc ServiceI) *Usecase {
	return &Usecase{
		svc: svc,
	}
}

func (u *Usecase) GetVersion() string {
	return u.svc.GetVersion()
}

func (u *Usecase) Get() []byte {
	return u.svc.Get()
}

func (u *Usecase) Refresh() {
	u.svc.Refresh()
}
