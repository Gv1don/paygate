package bank

type Register3DSRequest struct {
	Amount      int64
	Currency    string
	OrderID     string
	ReturnURL   string
	CallbackURL string
}

type Register3DSResponse struct {
	ThreeDSServerTransID string
	AcsURL              string
	CReq                string
}

type Submit3DSRequest struct {
	BankSessionID        string
	CRes                 string
	ThreeDSServerTransID string
}

type Submit3DSResponse struct {
	Success bool
}

type Provider interface {
	Register3DS(req *Register3DSRequest) (*Register3DSResponse, error)
	Submit3DS(req *Submit3DSRequest) (*Submit3DSResponse, error)
}
