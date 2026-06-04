package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"paygate/backend/internal/models"
)

// PaymentServiceInterface определяет контракт сервиса платежей.
type PaymentServiceInterface interface {
	InitiatePayment(models.InitPaymentRequest) (*models.Payment, error)
	ConfirmPayment(string, string) (*models.Payment, error)
	CompletePayment(string) (*models.Payment, error)
	FailPayment(string, string) (*models.Payment, error)
	RefundPayment(string, int64) (*models.Payment, error)
	GetPayment(string) (*models.Payment, error)
}

// PaymentHandler собирает HTTP-обработчики для платежей.
type PaymentHandler struct {
	svc PaymentServiceInterface
}

// NewPaymentHandler создаёт новый PaymentHandler.
func NewPaymentHandler(svc PaymentServiceInterface) *PaymentHandler {
	return &PaymentHandler{svc: svc}
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

	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "order_id is required")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}
	if req.Currency == "" {
		req.Currency = "RUB"
	}

	payment, err := h.svc.InitiatePayment(req)
	if err != nil {
		log.Printf("init payment error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to initiate payment")
		return
	}

	resp := models.InitPaymentResponse{
		BankSessionID: payment.BankSessionID,
		ThreeDSURL:    payment.ThreeDSURL,
		Status:        string(payment.Status),
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

	if req.BankSessionID == "" || req.Token == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id and token are required")
		return
	}

	payment, err := h.svc.ConfirmPayment(req.BankSessionID, req.Token)
	if err != nil {
		log.Printf("confirm payment error: %v", err)
		code := http.StatusInternalServerError
		if err.Error() == "invalid 3ds token" {
			code = http.StatusForbidden
		} else if err.Error() == "payment not found" {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID: payment.BankSessionID,
		Status:        string(payment.Status),
		Amount:        payment.Amount,
		Currency:      payment.Currency,
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

	sessionID := r.URL.Query().Get("bank_session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id query param is required")
		return
	}

	payment, err := h.svc.GetPayment(sessionID)
	if err != nil {
		log.Printf("get payment error: %v", err)
		code := http.StatusInternalServerError
		if err.Error() == "payment not found" {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID: payment.BankSessionID,
		Status:        string(payment.Status),
		Amount:        payment.Amount,
		Currency:      payment.Currency,
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

	if req.BankSessionID == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id is required")
		return
	}
	if req.Amount <= 0 {
		writeError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	payment, err := h.svc.RefundPayment(req.BankSessionID, req.Amount)
	if err != nil {
		log.Printf("refund payment error: %v", err)
		code := http.StatusInternalServerError
		if err.Error() == "payment not found" {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}

	resp := models.PaymentStatusResponse{
		BankSessionID: payment.BankSessionID,
		Status:        string(payment.Status),
		Amount:        payment.Amount,
		Currency:      payment.Currency,
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

	if payload.BankSessionID == "" || payload.Status == "" {
		writeError(w, http.StatusBadRequest, "bank_session_id and status are required")
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
		if err.Error() == "payment not found" {
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

// RegisterRoutes регистрирует все маршруты на переданном мультиплексоре.
func RegisterRoutes(mux *http.ServeMux, h *PaymentHandler) {
	mux.HandleFunc("/api/v1/payments/init", h.HandleInitPayment)
	mux.HandleFunc("/api/v1/payments/confirm", h.HandleConfirmPayment)
	mux.HandleFunc("/api/v1/payments/status", h.HandleGetPaymentStatus)
	mux.HandleFunc("/api/v1/payments/refund", h.HandleRefundPayment)
	mux.HandleFunc("/api/v1/webhooks/bank", h.HandleWebhook)
	mux.HandleFunc("/health", handleHealth)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}
