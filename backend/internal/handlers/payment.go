package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"paygate/backend/internal/models"
	"paygate/backend/internal/service"
)

type PaymentServiceInterface interface {
	InitiatePayment(models.InitPaymentRequest) (*models.Payment, error)
	ConfirmPayment(string, string) (*models.Payment, error)
	CompletePayment(string) (*models.Payment, error)
	FailPayment(string, string) (*models.Payment, error)
	RefundPayment(string, int64) (*models.Payment, error)
	GetPayment(string) (*models.Payment, error)
	Process3DSReturn(string, string) (*models.Payment, error)
	VerifyWebhookSignature(models.WebhookPayload) bool
}

type PaymentHandler struct {
	svc         PaymentServiceInterface
	frontendURL string
}

func NewPaymentHandler(svc PaymentServiceInterface, frontendURL string) *PaymentHandler {
	return &PaymentHandler{svc: svc, frontendURL: frontendURL}
}

var validCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func sanitizeString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// HandleInitPayment — POST /api/v1/payments/init
//
//	@Summary      Инициализация платежа
//	@Description  Создаёт новый платёж и возвращает session_id для 3DS-аутентификации
//	@Tags         Payments
//	@Accept       json
//	@Produce      json
//	@Param        body body models.InitPaymentRequest true "Данные платежа"
//	@Success      200 {object} models.InitPaymentResponse
//	@Failure      400 {object} map[string]string
//	@Failure      500 {object} map[string]string
//	@Router       /api/v1/payments/init [post]
func (h *PaymentHandler) HandleInitPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.InitPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	req.OrderID = sanitizeString(req.OrderID, 128)
	req.UserID = sanitizeString(req.UserID, 128)
	req.Currency = strings.ToUpper(strings.TrimSpace(req.Currency))
	req.ReturnURL = sanitizeString(req.ReturnURL, 2048)
	req.IdempotencyKey = sanitizeString(req.IdempotencyKey, 256)

	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "order_id is required")
		return
	}
	if req.Amount <= 0 || req.Amount > 999999999 {
		writeError(w, http.StatusBadRequest, "amount must be between 1 and 999999999")
		return
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}
	if !validCurrency.MatchString(req.Currency) {
		writeError(w, http.StatusBadRequest, "currency must be a 3-letter code (e.g. RUB, USD)")
		return
	}
	if req.ReturnURL != "" && !strings.HasPrefix(req.ReturnURL, "https://") {
		writeError(w, http.StatusBadRequest, "return_url must be a valid HTTPS URL")
		return
	}

	payment, err := h.svc.InitiatePayment(req)
	if err != nil {
		log.Printf("init payment error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to initiate payment")
		return
	}

	resp := models.InitPaymentResponse{
		BankSessionID:        payment.BankSessionID,
		PaymentURL:           h.frontendURL + "/pay?session=" + payment.BankSessionID,
		ThreeDSURL:           payment.ThreeDSURL,
		ThreeDSServerTransID: payment.ThreeDSServerTransID,
		Status:               string(payment.Status),
	}

	writeJSON(w, http.StatusOK, resp)
}

// Handle3DSReturn — POST /api/v1/payments/3ds-return
//
//	@Summary      Обработка возврата 3DS
//	@Description  Принимает результат 3DS-аутентификации от ACS банка
//	@Tags         Payments
//	@Accept       json
//	@Produce      json
//	@Param        body body models.ThreeDSReturnRequest true "Результат 3DS"
//	@Success      200 {object} models.PaymentStatusResponse
//	@Failure      400 {object} map[string]string
//	@Failure      404 {object} map[string]string
//	@Router       /api/v1/payments/3ds-return [post]
func (h *PaymentHandler) Handle3DSReturn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.ThreeDSReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	req.BankSessionID = sanitizeString(req.BankSessionID, 256)
	req.CRes = sanitizeString(req.CRes, 4096)

	if req.BankSessionID == "" || req.CRes == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id and cres are required")
		return
	}

	payment, err := h.svc.Process3DSReturn(req.BankSessionID, req.CRes)
	if err != nil {
		log.Printf("3ds return processing error: %v", err)
		code := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrPaymentNotFound):
			code = http.StatusNotFound
		case errors.Is(err, service.ErrInvalidTransition):
			code = http.StatusConflict
		case errors.Is(err, service.ErrInvalidCRes):
			code = http.StatusForbidden
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID:        payment.BankSessionID,
		Status:               string(payment.Status),
		Amount:               payment.Amount,
		Currency:             payment.Currency,
		ThreeDSServerTransID: payment.ThreeDSServerTransID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleConfirmPayment — POST /api/v1/payments/confirm
//
//	@Summary      Подтверждение платежа (3DS)
//	@Description  Подтверждает платёж после успешной 3DS-аутентификации
//	@Tags         Payments
//	@Accept       json
//	@Produce      json
//	@Param        body body models.ConfirmPaymentRequest true "3DS токен"
//	@Success      200 {object} models.PaymentStatusResponse
//	@Failure      400 {object} map[string]string
//	@Failure      403 {object} map[string]string
//	@Failure      404 {object} map[string]string
//	@Router       /api/v1/payments/confirm [post]
func (h *PaymentHandler) HandleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.ConfirmPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	req.BankSessionID = sanitizeString(req.BankSessionID, 256)
	req.Token = sanitizeString(req.Token, 512)

	if req.BankSessionID == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id and token are required")
		return
	}

	payment, err := h.svc.ConfirmPayment(req.BankSessionID, req.Token)
	if err != nil {
		log.Printf("confirm payment error: %v", err)
		code := http.StatusInternalServerError
		switch {
		case errors.Is(err, service.ErrInvalidToken):
			code = http.StatusForbidden
		case errors.Is(err, service.ErrPaymentNotFound):
			code = http.StatusNotFound
		case errors.Is(err, service.ErrInvalidTransition):
			code = http.StatusConflict
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID:        payment.BankSessionID,
		Status:               string(payment.Status),
		Amount:               payment.Amount,
		Currency:             payment.Currency,
		ThreeDSServerTransID: payment.ThreeDSServerTransID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleGetPaymentStatus — GET /api/v1/payments/status
//
//	@Summary      Статус платежа
//	@Description  Возвращает текущий статус платежа по bank_session_id
//	@Tags         Payments
//	@Produce      json
//	@Param        bank_session_id query string true "Идентификатор сессии банка"
//	@Success      200 {object} models.PaymentStatusResponse
//	@Failure      400 {object} map[string]string
//	@Failure      404 {object} map[string]string
//	@Router       /api/v1/payments/status [get]
func (h *PaymentHandler) HandleGetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sessionID := sanitizeString(r.URL.Query().Get("bank_session_id"), 256)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id query param is required")
		return
	}

	payment, err := h.svc.GetPayment(sessionID)
	if err != nil {
		log.Printf("get payment error: %v", err)
		code := http.StatusInternalServerError
		if errors.Is(err, service.ErrPaymentNotFound) {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID:        payment.BankSessionID,
		Status:               string(payment.Status),
		Amount:               payment.Amount,
		Currency:             payment.Currency,
		OrderID:              payment.OrderID,
		ReturnURL:            payment.ReturnURL,
		ThreeDSServerTransID: payment.ThreeDSServerTransID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleRefundPayment — POST /api/v1/payments/refund
//
//	@Summary      Возврат платежа
//	@Description  Инициирует возврат средств по платежу
//	@Tags         Payments
//	@Accept       json
//	@Produce      json
//	@Param        body body models.RefundRequest true "Данные возврата"
//	@Success      200 {object} models.PaymentStatusResponse
//	@Failure      400 {object} map[string]string
//	@Failure      404 {object} map[string]string
//	@Router       /api/v1/payments/refund [post]
func (h *PaymentHandler) HandleRefundPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	req.BankSessionID = sanitizeString(req.BankSessionID, 256)

	if req.BankSessionID == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id is required")
		return
	}
	if req.Amount <= 0 || req.Amount > 999999999 {
		writeError(w, http.StatusBadRequest, "amount must be between 1 and 999999999")
		return
	}

	payment, err := h.svc.RefundPayment(req.BankSessionID, req.Amount)
	if err != nil {
		log.Printf("refund payment error: %v", err)
		code := http.StatusInternalServerError
		if errors.Is(err, service.ErrPaymentNotFound) {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID:        payment.BankSessionID,
		Status:               string(payment.Status),
		Amount:               payment.Amount,
		Currency:             payment.Currency,
		ThreeDSServerTransID: payment.ThreeDSServerTransID,
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleWebhook — POST /api/v1/webhooks/bank
//
//	@Summary      Webhook от банка
//	@Description  Обрабатывает входящие уведомления от банка (статус платежа)
//	@Tags         Webhooks
//	@Accept       json
//	@Produce      json
//	@Param        body body models.WebhookPayload true "Уведомление от банка"
//	@Success      200 {object} map[string]string
//	@Failure      400 {object} map[string]string
//	@Failure      404 {object} map[string]string
//	@Failure      500 {object} map[string]string
//	@Router       /api/v1/webhooks/bank [post]
func (h *PaymentHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var payload models.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	payload.BankSessionID = sanitizeString(payload.BankSessionID, 256)
	payload.Status = sanitizeString(payload.Status, 64)

	if payload.BankSessionID == "" || payload.Status == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id and status are required")
		return
	}

	if !h.svc.VerifyWebhookSignature(payload) {
		writeError(w, http.StatusForbidden, "invalid webhook signature")
		return
	}

	var err error
	switch models.PaymentStatus(payload.Status) {
	case models.StatusCompleted:
		_, err = h.svc.CompletePayment(payload.BankSessionID)
	case models.StatusFailed:
		_, err = h.svc.FailPayment(payload.BankSessionID, "bank declined")
	default:
		writeError(w, http.StatusBadRequest, "unsupported webhook status")
		return
	}

	if err != nil {
		log.Printf("webhook processing error: %v", err)
		if errors.Is(err, service.ErrPaymentNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to process webhook")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, statusCode int, msg string) {
	writeJSON(w, statusCode, map[string]string{"error": msg})
}

func RegisterRoutes(mux *http.ServeMux, h *PaymentHandler) {
	mux.HandleFunc("/api/v1/payments/init", h.HandleInitPayment)
	mux.HandleFunc("/api/v1/payments/3ds-return", h.Handle3DSReturn)
	mux.HandleFunc("/api/v1/payments/confirm", h.HandleConfirmPayment)
	mux.HandleFunc("/api/v1/payments/status", h.HandleGetPaymentStatus)
	mux.HandleFunc("/api/v1/payments/refund", h.HandleRefundPayment)
	mux.HandleFunc("/api/v1/webhooks/bank", h.HandleWebhook)
	mux.HandleFunc("/health", handleHealth)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
