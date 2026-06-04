package models

import "time"

// PaymentStatus представляет жизненный цикл статусов платежа.
type PaymentStatus string

const (
	StatusInitiated  PaymentStatus = "INITIATED"
	StatusPending3DS PaymentStatus = "PENDING_3DS"
	StatusConfirmed  PaymentStatus = "CONFIRMED"
	StatusCompleted  PaymentStatus = "COMPLETED"
	StatusFailed     PaymentStatus = "FAILED"
	StatusRefunded   PaymentStatus = "REFUNDED"
)

// Payment — основная модель платежа, хранимая в ScyllaDB.
type Payment struct {
	BankSessionID string        `json:"bank_session_id"`
	OrderID       string        `json:"order_id"`
	UserID        string        `json:"user_id"`
	Amount        int64         `json:"amount"`
	Currency      string        `json:"currency"`
	Status        PaymentStatus `json:"status"`
	ThreeDSURL    string        `json:"three_ds_url,omitempty"`
	FailReason    string        `json:"fail_reason,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

// IsTerminal возвращает true, если статус является терминальным.
func (p *Payment) IsTerminal() bool {
	switch p.Status {
	case StatusCompleted, StatusFailed, StatusRefunded:
		return true
	default:
		return false
	}
}

// CanTransitionTo проверяет, допустим ли переход в указанный статус.
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

// InitPaymentRequest — запрос на инициализацию платежа.
type InitPaymentRequest struct {
	OrderID  string `json:"order_id" example:"order-12345"`
	UserID   string `json:"user_id"  example:"user-67890"`
	Amount   int64  `json:"amount"   example:"1000"`
	Currency string `json:"currency" example:"RUB"`
}

// InitPaymentResponse — ответ после инициализации платежа.
type InitPaymentResponse struct {
	BankSessionID string `json:"bank_session_id" example:"bank_sys_abc123def456"`
	ThreeDSURL    string `json:"three_ds_url,omitempty" example:"https://3ds.bank.com/auth/sess_123"`
	Status        string `json:"status" example:"INITIATED"`
}

// ConfirmPaymentRequest — запрос на подтверждение платежа (3DS).
type ConfirmPaymentRequest struct {
	BankSessionID string `json:"bank_session_id" example:"bank_sys_abc123def456"`
	Token         string `json:"token" example:"3ds_token_xyz789"`
}

// PaymentStatusResponse — ответ о статусе платежа.
type PaymentStatusResponse struct {
	BankSessionID string `json:"bank_session_id" example:"bank_sys_abc123def456"`
	Status        string `json:"status" example:"COMPLETED"`
	Amount        int64  `json:"amount" example:"1000"`
	Currency      string `json:"currency" example:"RUB"`
}

// RefundRequest — запрос на возврат платежа.
type RefundRequest struct {
	BankSessionID string `json:"bank_session_id" example:"bank_sys_abc123def456"`
	Amount        int64  `json:"amount" example:"500"`
}

// WebhookPayload — входящий вебхук от банка.
type WebhookPayload struct {
	BankSessionID string `json:"bank_session_id"`
	Status        string `json:"status"`
	Timestamp     string `json:"timestamp"`
	Signature     string `json:"signature"`
}
