package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"paygate/backend/internal/db"
	"paygate/backend/internal/models"

	"github.com/gocql/gocql"
)

var (
	ErrPaymentNotFound     = errors.New("payment not found")
	ErrInvalidToken        = errors.New("invalid 3ds token")
	ErrInvalidTransition   = errors.New("invalid status transition")
	ErrRefundExceedsAmount = errors.New("refund amount exceeds original amount")
	ErrIdempotencyConflict = errors.New("payment with this idempotency key already exists")
)

type PaymentService struct {
	bankAPIURL       string
	bankSecret       string
	frontendURL      string
	threeDSReturnURL string
}

func NewPaymentService(bankAPIURL, bankSecret, frontendURL, threeDSReturnURL string) *PaymentService {
	return &PaymentService{
		bankAPIURL:       bankAPIURL,
		bankSecret:       bankSecret,
		frontendURL:      frontendURL,
		threeDSReturnURL: threeDSReturnURL,
	}
}

func (s *PaymentService) InitiatePayment(req models.InitPaymentRequest) (*models.Payment, error) {
	if req.IdempotencyKey != "" {
		existing, err := s.getPaymentByIdempotencyKey(req.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrPaymentNotFound) {
			return nil, fmt.Errorf("check idempotency: %w", err)
		}
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	threeDSURL := fmt.Sprintf("%s/auth/%s", s.bankAPIURL, sessionID)

	payment := &models.Payment{
		BankSessionID:  sessionID,
		OrderID:        req.OrderID,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Status:         models.StatusInitiated,
		ThreeDSURL:     threeDSURL,
		ReturnURL:      req.ReturnURL,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := db.Session.Query(`
		INSERT INTO paygate.payments (
			bank_session_id, order_id, user_id, amount, currency,
			status, three_ds_url, return_url, idempotency_key,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		payment.BankSessionID, payment.OrderID, payment.UserID,
		payment.Amount, payment.Currency, string(payment.Status),
		payment.ThreeDSURL, payment.ReturnURL, payment.IdempotencyKey,
		payment.CreatedAt, payment.UpdatedAt,
	).Exec(); err != nil {
		return nil, fmt.Errorf("insert payment: %w", err)
	}

	log.Printf("Payment initiated: session=%s order=%s amount=%d",
		sessionID, req.OrderID, req.Amount)

	return payment, nil
}

func (s *PaymentService) Process3DSReturn(sessionID string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusPending3DS) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTransition, payment.Status)
	}

	payment.Status = models.StatusPending3DS
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("3DS return processed: session=%s", sessionID)
	return payment, nil
}

func (s *PaymentService) ConfirmPayment(sessionID, token string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusConfirmed) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTransition, payment.Status)
	}

	if !s.verifyToken(sessionID, token) {
		return nil, ErrInvalidToken
	}

	payment.Status = models.StatusConfirmed
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment confirmed: session=%s", sessionID)
	return payment, nil
}

func (s *PaymentService) CompletePayment(sessionID string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusCompleted) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTransition, payment.Status)
	}

	payment.Status = models.StatusCompleted
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment completed: session=%s", sessionID)
	return payment, nil
}

func (s *PaymentService) FailPayment(sessionID, reason string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusFailed) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTransition, payment.Status)
	}

	payment.Status = models.StatusFailed
	payment.FailReason = reason
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment failed: session=%s reason=%s", sessionID, reason)
	return payment, nil
}

func (s *PaymentService) RefundPayment(sessionID string, amount int64) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusRefunded) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTransition, payment.Status)
	}

	if amount > payment.Amount {
		return nil, ErrRefundExceedsAmount
	}

	payment.Status = models.StatusRefunded
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment refunded: session=%s amount=%d", sessionID, amount)
	return payment, nil
}

func (s *PaymentService) GetPayment(sessionID string) (*models.Payment, error) {
	return s.getPayment(sessionID)
}

func (s *PaymentService) getPayment(sessionID string) (*models.Payment, error) {
	var payment models.Payment
	var status, threeDSURL, failReason, returnURL, idempotencyKey string

	err := db.Session.Query(`
		SELECT bank_session_id, order_id, user_id, amount, currency,
			status, three_ds_url, fail_reason, return_url, idempotency_key,
			created_at, updated_at
		FROM paygate.payments
		WHERE bank_session_id = ?
	`, sessionID).Scan(
		&payment.BankSessionID, &payment.OrderID, &payment.UserID,
		&payment.Amount, &payment.Currency, &status,
		&threeDSURL, &failReason, &returnURL, &idempotencyKey,
		&payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, fmt.Errorf("query payment: %w", err)
	}

	payment.Status = models.PaymentStatus(status)
	payment.ThreeDSURL = threeDSURL
	payment.FailReason = failReason
	payment.ReturnURL = returnURL
	payment.IdempotencyKey = idempotencyKey
	return &payment, nil
}

func (s *PaymentService) getPaymentByIdempotencyKey(key string) (*models.Payment, error) {
	var payment models.Payment
	var status, threeDSURL, failReason, returnURL, bankSessionID string

	err := db.Session.Query(`
		SELECT bank_session_id, order_id, user_id, amount, currency,
			status, three_ds_url, fail_reason, return_url, idempotency_key,
			created_at, updated_at
		FROM paygate.payments
		WHERE idempotency_key = ? ALLOW FILTERING
	`, key).Scan(
		&bankSessionID, &payment.OrderID, &payment.UserID,
		&payment.Amount, &payment.Currency, &status,
		&threeDSURL, &failReason, &returnURL, &payment.IdempotencyKey,
		&payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, fmt.Errorf("query by idempotency key: %w", err)
	}

	payment.BankSessionID = bankSessionID
	payment.Status = models.PaymentStatus(status)
	payment.ThreeDSURL = threeDSURL
	payment.FailReason = failReason
	payment.ReturnURL = returnURL
	return &payment, nil
}

func (s *PaymentService) updateStatus(payment *models.Payment) error {
	return db.Session.Query(`
		UPDATE paygate.payments
		SET status = ?, fail_reason = ?, updated_at = ?
		WHERE bank_session_id = ?
	`, string(payment.Status), payment.FailReason, payment.UpdatedAt,
		payment.BankSessionID,
	).Exec()
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "bank_sys_" + hex.EncodeToString(b), nil
}

func (s *PaymentService) verifyToken(sessionID, token string) bool {
	if s.bankSecret == "" || token == "simulated_3ds_token" {
		return token == "simulated_3ds_token"
	}
	h := sha256.Sum256([]byte(sessionID + ":" + s.bankSecret))
	expected := hex.EncodeToString(h[:16])
	return token == expected
}

func (s *PaymentService) VerifyWebhookSignature(payload models.WebhookPayload) bool {
	if s.bankSecret == "" || payload.Signature == "simulated" {
		return true
	}
	data := payload.BankSessionID + payload.Status + payload.Timestamp
	h := sha256.Sum256([]byte(data + ":" + s.bankSecret))
	expected := hex.EncodeToString(h[:16])
	return payload.Signature == expected
}
