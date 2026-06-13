package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"paygate/backend/internal/models"
	"paygate/backend/internal/service"
)

type mockPaymentService struct {
	initiateFn        func(models.InitPaymentRequest) (*models.Payment, error)
	confirmFn         func(string, string) (*models.Payment, error)
	completeFn        func(string) (*models.Payment, error)
	failFn            func(string, string) (*models.Payment, error)
	refundFn          func(string, int64) (*models.Payment, error)
	getPaymentFn      func(string) (*models.Payment, error)
	process3DSReturnFn func(string) (*models.Payment, error)
	verifyWebhookFn   func(models.WebhookPayload) bool
}

func (m *mockPaymentService) InitiatePayment(req models.InitPaymentRequest) (*models.Payment, error) {
	return m.initiateFn(req)
}
func (m *mockPaymentService) ConfirmPayment(id, token string) (*models.Payment, error) {
	return m.confirmFn(id, token)
}
func (m *mockPaymentService) CompletePayment(id string) (*models.Payment, error) {
	return m.completeFn(id)
}
func (m *mockPaymentService) FailPayment(id, reason string) (*models.Payment, error) {
	return m.failFn(id, reason)
}
func (m *mockPaymentService) RefundPayment(id string, amount int64) (*models.Payment, error) {
	return m.refundFn(id, amount)
}
func (m *mockPaymentService) GetPayment(id string) (*models.Payment, error) {
	return m.getPaymentFn(id)
}
func (m *mockPaymentService) Process3DSReturn(id string) (*models.Payment, error) {
	return m.process3DSReturnFn(id)
}
func (m *mockPaymentService) VerifyWebhookSignature(p models.WebhookPayload) bool {
	return m.verifyWebhookFn(p)
}

func now() time.Time {
	return time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
}

func basePayment() *models.Payment {
	return &models.Payment{
		BankSessionID: "bank_sys_test123",
		OrderID:       "order-1",
		UserID:        "user-1",
		Amount:        1000,
		Currency:      "RUB",
		Status:        models.StatusInitiated,
		ThreeDSURL:    "https://bank.com/auth/bank_sys_test123",
		CreatedAt:     now(),
		UpdatedAt:     now(),
	}
}

func makeRequest(method, url, body string) *http.Request {
	req := httptest.NewRequest(method, url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHandleInitPayment_Success(t *testing.T) {
	mock := &mockPaymentService{
		initiateFn: func(req models.InitPaymentRequest) (*models.Payment, error) {
			p := basePayment()
			p.OrderID = req.OrderID
			p.UserID = req.UserID
			p.Amount = req.Amount
			p.Currency = req.Currency
			return p, nil
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/init",
		`{"order_id":"o1","user_id":"u1","amount":500,"currency":"RUB"}`)
	w := httptest.NewRecorder()
	handler.HandleInitPayment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.InitPaymentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatal("failed to decode response:", err)
	}
	if resp.BankSessionID == "" {
		t.Error("expected non-empty bank_session_id")
	}
	if resp.Status != string(models.StatusInitiated) {
		t.Errorf("expected status INITIATED, got %s", resp.Status)
	}
}

func TestHandleInitPayment_BadRequest(t *testing.T) {
	mock := &mockPaymentService{}
	handler := &PaymentHandler{svc: mock}

	tests := []struct {
		name string
		body string
	}{
		{"empty json", `{}`},
		{"missing order_id", `{"amount":100}`},
		{"negative amount", `{"order_id":"o1","amount":-100}`},
		{"zero amount", `{"order_id":"o1","amount":0}`},
		{"invalid json", `not json`},
		{"amount too large", `{"order_id":"o1","amount":1000000000}`},
		{"invalid currency", `{"order_id":"o1","amount":100,"currency":"RUBB"}`},
		{"non-https return_url", `{"order_id":"o1","amount":100,"return_url":"http://evil.com"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := makeRequest(http.MethodPost, "/api/v1/payments/init", tt.body)
			w := httptest.NewRecorder()
			handler.HandleInitPayment(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleInitPayment_MethodNotAllowed(t *testing.T) {
	handler := &PaymentHandler{svc: &mockPaymentService{}}

	req := makeRequest(http.MethodGet, "/api/v1/payments/init", "")
	w := httptest.NewRecorder()
	handler.HandleInitPayment(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleInitPayment_ServiceError(t *testing.T) {
	mock := &mockPaymentService{
		initiateFn: func(req models.InitPaymentRequest) (*models.Payment, error) {
			return nil, errors.New("db error")
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/init",
		`{"order_id":"o1","amount":500}`)
	w := httptest.NewRecorder()
	handler.HandleInitPayment(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleConfirmPayment_Success(t *testing.T) {
	mock := &mockPaymentService{
		confirmFn: func(id, token string) (*models.Payment, error) {
			p := basePayment()
			p.Status = models.StatusConfirmed
			return p, nil
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/confirm",
		`{"bank_session_id":"test","token":"valid_token"}`)
	w := httptest.NewRecorder()
	handler.HandleConfirmPayment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.PaymentStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != string(models.StatusConfirmed) {
		t.Errorf("expected CONFIRMED, got %s", resp.Status)
	}
}

func TestHandleConfirmPayment_InvalidToken(t *testing.T) {
	mock := &mockPaymentService{
		confirmFn: func(id, token string) (*models.Payment, error) {
			return nil, service.ErrInvalidToken
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/confirm",
		`{"bank_session_id":"test","token":"bad_token"}`)
	w := httptest.NewRecorder()
	handler.HandleConfirmPayment(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestHandleConfirmPayment_NotFound(t *testing.T) {
	mock := &mockPaymentService{
		confirmFn: func(id, token string) (*models.Payment, error) {
			return nil, service.ErrPaymentNotFound
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/confirm",
		`{"bank_session_id":"test","token":"token"}`)
	w := httptest.NewRecorder()
	handler.HandleConfirmPayment(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetPaymentStatus_Success(t *testing.T) {
	mock := &mockPaymentService{
		getPaymentFn: func(id string) (*models.Payment, error) {
			p := basePayment()
			p.Status = models.StatusCompleted
			return p, nil
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodGet, "/api/v1/payments/status?bank_session_id=test123", "")
	w := httptest.NewRecorder()
	handler.HandleGetPaymentStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp models.PaymentStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.BankSessionID != "bank_sys_test123" {
		t.Errorf("expected bank_sys_test123, got %s", resp.BankSessionID)
	}
}

func TestHandleGetPaymentStatus_NotFound(t *testing.T) {
	mock := &mockPaymentService{
		getPaymentFn: func(id string) (*models.Payment, error) {
			return nil, service.ErrPaymentNotFound
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodGet, "/api/v1/payments/status?bank_session_id=nonexistent", "")
	w := httptest.NewRecorder()
	handler.HandleGetPaymentStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetPaymentStatus_MissingParam(t *testing.T) {
	handler := &PaymentHandler{svc: &mockPaymentService{}}

	req := makeRequest(http.MethodGet, "/api/v1/payments/status", "")
	w := httptest.NewRecorder()
	handler.HandleGetPaymentStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleRefundPayment_Success(t *testing.T) {
	mock := &mockPaymentService{
		refundFn: func(id string, amount int64) (*models.Payment, error) {
			p := basePayment()
			p.Status = models.StatusRefunded
			p.Amount = amount
			return p, nil
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/refund",
		`{"bank_session_id":"test","amount":500}`)
	w := httptest.NewRecorder()
	handler.HandleRefundPayment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp models.PaymentStatusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Status != string(models.StatusRefunded) {
		t.Errorf("expected REFUNDED, got %s", resp.Status)
	}
	if resp.Amount != 500 {
		t.Errorf("expected amount 500, got %d", resp.Amount)
	}
}

func TestHandleRefundPayment_InvalidAmount(t *testing.T) {
	handler := &PaymentHandler{svc: &mockPaymentService{}}

	req := makeRequest(http.MethodPost, "/api/v1/payments/refund",
		`{"bank_session_id":"test","amount":-100}`)
	w := httptest.NewRecorder()
	handler.HandleRefundPayment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleRefundPayment_NotFound(t *testing.T) {
	mock := &mockPaymentService{
		refundFn: func(id string, amount int64) (*models.Payment, error) {
			return nil, service.ErrPaymentNotFound
		},
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/payments/refund",
		`{"bank_session_id":"nonexistent","amount":100}`)
	w := httptest.NewRecorder()
	handler.HandleRefundPayment(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleWebhook_Completed(t *testing.T) {
	mock := &mockPaymentService{
		completeFn: func(id string) (*models.Payment, error) {
			p := basePayment()
			p.Status = models.StatusCompleted
			return p, nil
		},
		verifyWebhookFn: func(p models.WebhookPayload) bool { return true },
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/webhooks/bank",
		`{"bank_session_id":"test","status":"COMPLETED","timestamp":"2026-06-04T12:00:00Z","signature":"abc"}`)
	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleWebhook_Failed(t *testing.T) {
	mock := &mockPaymentService{
		failFn: func(id, reason string) (*models.Payment, error) {
			p := basePayment()
			p.Status = models.StatusFailed
			return p, nil
		},
		verifyWebhookFn: func(p models.WebhookPayload) bool { return true },
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/webhooks/bank",
		`{"bank_session_id":"test","status":"FAILED","timestamp":"2026-06-04T12:00:00Z","signature":"abc"}`)
	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleWebhook_UnsupportedStatus(t *testing.T) {
	handler := &PaymentHandler{svc: &mockPaymentService{
		verifyWebhookFn: func(p models.WebhookPayload) bool { return true },
	}}

	req := makeRequest(http.MethodPost, "/api/v1/webhooks/bank",
		`{"bank_session_id":"test","status":"PENDING"}`)
	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleWebhook_NotFound(t *testing.T) {
	mock := &mockPaymentService{
		completeFn: func(id string) (*models.Payment, error) {
			return nil, service.ErrPaymentNotFound
		},
		verifyWebhookFn: func(p models.WebhookPayload) bool { return true },
	}
	handler := &PaymentHandler{svc: mock}

	req := makeRequest(http.MethodPost, "/api/v1/webhooks/bank",
		`{"bank_session_id":"nonexistent","status":"COMPLETED"}`)
	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleWebhook_MethodNotAllowed(t *testing.T) {
	handler := &PaymentHandler{svc: &mockPaymentService{}}

	req := makeRequest(http.MethodGet, "/api/v1/webhooks/bank", "")
	w := httptest.NewRecorder()
	handler.HandleWebhook(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHealth(t *testing.T) {
	req := makeRequest(http.MethodGet, "/health", "")
	w := httptest.NewRecorder()
	handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "healthy" {
		t.Errorf("expected healthy, got %s", resp["status"])
	}
}
