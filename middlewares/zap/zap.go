// Package zapmw fornece um middleware de log com uber-go/zap.
// Fica em um módulo separado para que o zap só entre em quem o usa.
package zapmw

import (
	"context"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"go.uber.org/zap"
)

// LoggingMiddleware logs the start, end and duration of each task with zap.
func LoggingMiddleware(logger *zap.Logger) interfaces.Middleware {
	return func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, payload interfaces.Payload) error {
			start := time.Now()

			logger.Info("Starting task",
				zap.Any("payload", payload),
				zap.Time("inicio", start),
			)

			err := next(ctx, payload)

			if err != nil {
				logger.Error("Task falhou",
					zap.Any("payload", payload),
					zap.Duration("duration", time.Since(start)),
					zap.Error(err),
				)
			} else {
				logger.Info("Task completed successfully",
					zap.Any("payload", payload),
					zap.Duration("duration", time.Since(start)),
				)
			}

			return err
		}
	}
}
