package internal

import (
	"context"
	"log"
	"time"

	"github.com/Thauan/gotsk/interfaces"
)

func LoggingMiddleware(logger *log.Logger) interfaces.Middleware {
	return func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, payload interfaces.Payload) error {
			start := time.Now()

			logger.Printf("┌──────────────────────────────────────────────┐")
			logger.Printf("│ 🚀 Iniciando task com payload: %v", payload)
			logger.Printf("└──────────────────────────────────────────────┘")

			err := next(ctx, payload)

			duration := time.Since(start)

			logger.Printf("┌──────────────────────────────────────────────┐")
			if err != nil {
				logger.Printf("│ ❌ Task falhou: %v", err)
			} else {
				logger.Printf("│ ✅ Task finalizada com sucesso com payload: %v", payload)
			}
			logger.Printf("│ ⏱️  Duração: %s", duration)
			logger.Printf("└──────────────────────────────────────────────┘")

			return err
		}
	}
}
