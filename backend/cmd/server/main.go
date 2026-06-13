// Paygate — платёжный шлюз.
//
//	 API для инициализации, подтверждения и управления цифровыми платежами
//	 с поддержкой 3D Secure аутентификации.
//
//		 Schemes: http
//		 Host: localhost:8081
//		 BasePath: /
//		 Version: 1.0.0
//
//		 Consumes:
//		 - application/json
//
//		 Produces:
//		 - application/json
//
//	 swagger:meta
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "paygate/backend/docs"

	"paygate/backend/internal/bank"
	"paygate/backend/internal/config"
	"paygate/backend/internal/db"
	"paygate/backend/internal/handlers"
	"paygate/backend/internal/middleware"
	"paygate/backend/internal/service"

	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	cfg := config.Load()

	if err := db.Connect(cfg.ScyllaAddr, cfg.ScyllaKeyspace); err != nil {
		log.Fatalf("failed to connect to scylladb: %v", err)
	}
	defer db.Close()

	bankProvider := bank.NewMockProvider(cfg.BankAPIURL)
	svc := service.NewPaymentService(bankProvider, cfg.BankSecret, cfg.FrontendURL, cfg.ThreeDSReturnURL)
	handler := handlers.NewPaymentHandler(svc, cfg.FrontendURL)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, handler)

	mux.HandleFunc("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("http://"+cfg.SwaggerHost+"/swagger/doc.json"),
	))

	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	var wrapped http.Handler = mux
	wrapped = middleware.CORS(cfg.CORSOrigin)(wrapped)
	wrapped = rateLimiter.Middleware(wrapped)
	wrapped = middleware.Logging(wrapped)
	wrapped = middleware.Recovery(wrapped)

	server := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %s, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("forced shutdown: %v", err)
	}
	log.Printf("Server stopped gracefully")
}
