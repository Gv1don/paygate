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
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "paygate/backend/docs"

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

	svc := service.NewPaymentService(cfg.BankAPIURL, cfg.BankSecret)
	handler := handlers.NewPaymentHandler(svc)

	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux, handler)

	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	var wrapped http.Handler = mux
	wrapped = middleware.CORS(wrapped)
	wrapped = middleware.Logging(wrapped)
	wrapped = middleware.Recovery(wrapped)

	server := &http.Server{
		Addr:    cfg.Addr(),
		Handler: wrapped,
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
}
