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

// PaymentService содержит бизнес-логику платежей.
type PaymentService struct {
	bankAPIURL string
	bankSecret string
}

// NewPaymentService создаёт новый сервис платежей.
func NewPaymentService(bankAPIURL, bankSecret string) *PaymentService {
	return &PaymentService{
		bankAPIURL: bankAPIURL,
		bankSecret: bankSecret,
	}
}

// InitiatePayment создаёт новый платёж и возвращает session_id для 3DS.
func (s *PaymentService) InitiatePayment(req models.InitPaymentRequest) (*models.Payment, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	threeDSURL := fmt.Sprintf("%s/auth/%s", s.bankAPIURL, sessionID)

	payment := &models.Payment{
		BankSessionID: sessionID,
		OrderID:       req.OrderID,
		UserID:        req.UserID,
		Amount:        req.Amount,
		Currency:      req.Currency,
		Status:        models.StatusInitiated,
		ThreeDSURL:    threeDSURL,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := db.Session.Query(`
		INSERT INTO paygate.payments (
			bank_session_id, order_id, user_id, amount, currency,
			status, three_ds_url, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		payment.BankSessionID, payment.OrderID, payment.UserID,
		payment.Amount, payment.Currency, string(payment.Status),
		payment.ThreeDSURL, payment.CreatedAt, payment.UpdatedAt,
	).Exec(); err != nil {
		return nil, fmt.Errorf("insert payment: %w", err)
	}

	log.Printf("Payment initiated: session=%s order=%s amount=%d",
		sessionID, req.OrderID, req.Amount)

	return payment, nil
}

// ConfirmPayment подтверждает платёж после успешной 3DS-аутентификации.
func (s *PaymentService) ConfirmPayment(sessionID, token string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusConfirmed) {
		return nil, fmt.Errorf("cannot confirm payment in status %s", payment.Status)
	}

	if !s.verifyToken(sessionID, token) {
		return nil, errors.New("invalid 3ds token")
	}

	payment.Status = models.StatusConfirmed
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment confirmed: session=%s", sessionID)
	return payment, nil
}

// CompletePayment финализирует платёж (вызывается после списания средств).
func (s *PaymentService) CompletePayment(sessionID string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusCompleted) {
		return nil, fmt.Errorf("cannot complete payment in status %s", payment.Status)
	}

	payment.Status = models.StatusCompleted
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment completed: session=%s", sessionID)
	return payment, nil
}

// FailPayment переводит платёж в статус FAILED.
func (s *PaymentService) FailPayment(sessionID, reason string) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusFailed) {
		return nil, fmt.Errorf("cannot fail payment in status %s", payment.Status)
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

// RefundPayment инициирует возврат средств.
func (s *PaymentService) RefundPayment(sessionID string, amount int64) (*models.Payment, error) {
	payment, err := s.getPayment(sessionID)
	if err != nil {
		return nil, err
	}

	if !payment.CanTransitionTo(models.StatusRefunded) {
		return nil, fmt.Errorf("cannot refund payment in status %s", payment.Status)
	}

	if amount > payment.Amount {
		return nil, errors.New("refund amount exceeds original amount")
	}

	payment.Status = models.StatusRefunded
	payment.UpdatedAt = time.Now().UTC()

	if err := s.updateStatus(payment); err != nil {
		return nil, fmt.Errorf("update payment status: %w", err)
	}

	log.Printf("Payment refunded: session=%s amount=%d", sessionID, amount)
	return payment, nil
}

// GetPayment возвращает платёж по session_id.
func (s *PaymentService) GetPayment(sessionID string) (*models.Payment, error) {
	return s.getPayment(sessionID)
}

func (s *PaymentService) getPayment(sessionID string) (*models.Payment, error) {
	var payment models.Payment
	var status, threeDSURL, failReason string

	err := db.Session.Query(`
		SELECT bank_session_id, order_id, user_id, amount, currency,
			status, three_ds_url, fail_reason, created_at, updated_at
		FROM paygate.payments
		WHERE bank_session_id = ?
	`, sessionID).Scan(
		&payment.BankSessionID, &payment.OrderID, &payment.UserID,
		&payment.Amount, &payment.Currency, &status,
		&threeDSURL, &failReason,
		&payment.CreatedAt, &payment.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, gocql.ErrNotFound) {
			return nil, errors.New("payment not found")
		}
		return nil, fmt.Errorf("query payment: %w", err)
	}

	payment.Status = models.PaymentStatus(status)
	payment.ThreeDSURL = threeDSURL
	payment.FailReason = failReason
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
	h := sha256.Sum256([]byte(sessionID + ":" + s.bankSecret))
	expected := hex.EncodeToString(h[:16])
	return token == expected
}
