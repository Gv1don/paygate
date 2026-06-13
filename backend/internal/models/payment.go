package models

import "time"

type PaymentStatus string

const (
	StatusInitiated  PaymentStatus = "INITIATED"
	StatusPending3DS PaymentStatus = "PENDING_3DS"
	StatusConfirmed  PaymentStatus = "CONFIRMED"
	StatusCompleted  PaymentStatus = "COMPLETED"
	StatusFailed     PaymentStatus = "FAILED"
	StatusRefunded   PaymentStatus = "REFUNDED"
)

type Payment struct {
	BankSessionID  string        `json:"bank_session_id"`
	OrderID        string        `json:"order_id"`
	UserID         string        `json:"user_id"`
	Amount         int64         `json:"amount"`
	Currency       string        `json:"currency"`
	Status         PaymentStatus `json:"status"`
	ThreeDSURL     string        `json:"three_ds_url,omitempty"`
	FailReason     string        `json:"fail_reason,omitempty"`
	ReturnURL      string        `json:"return_url,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

func (p *Payment) IsTerminal() bool {
	switch p.Status {
	case StatusCompleted, StatusFailed, StatusRefunded:
		return true
	default:
		return false
	}
}

func (p *Payment) CanTransitionTo(target PaymentStatus) bool {
	transitions := map[PaymentStatus][]PaymentStatus{
		StatusInitiated:  {StatusPending3DS, StatusFailed},
		StatusPending3DS: {StatusConfirmed, StatusFailed},
		StatusConfirmed:  {StatusCompleted, StatusFailed},
		StatusCompleted:  {StatusRefunded},
		StatusFailed:     {},
		StatusRefunded:   {},
	}
	allowed, ok := transitions[p.Status]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == target {
			return true
		}
	}
	return false
}

type InitPaymentRequest struct {
	OrderID        string `json:"order_id"`
	UserID         string `json:"user_id"`
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	ReturnURL      string `json:"return_url,omitempty"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

type InitPaymentResponse struct {
	BankSessionID string `json:"bank_session_id"`
	PaymentURL    string `json:"payment_url"`
	ThreeDSURL    string `json:"three_ds_url,omitempty"`
	Status        string `json:"status"`
}

type ThreeDSReturnRequest struct {
	BankSessionID string `json:"bank_session_id"`
	CRes          string `json:"cres"`
}

type ConfirmPaymentRequest struct {
	BankSessionID string `json:"bank_session_id"`
	Token         string `json:"token"`
}

type PaymentStatusResponse struct {
	BankSessionID string `json:"bank_session_id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	OrderID       string `json:"order_id"`
	ReturnURL     string `json:"return_url,omitempty"`
}

type RefundRequest struct {
	BankSessionID string `json:"bank_session_id"`
	Amount        int64  `json:"amount"`
}

type WebhookPayload struct {
	BankSessionID string `json:"bank_session_id"`
	Status        string `json:"status"`
	Timestamp     string `json:"timestamp"`
	Signature     string `json:"signature"`
}
