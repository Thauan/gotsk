package internal

import (
	"context"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"go.uber.org/zap"
)

func ZapLoggingMiddleware(logger *zap.Logger) interfaces.Middleware {
	return func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, payload interfaces.Payload) error {
			start := time.Now()

			logger.Info("Iniciando task",
				zap.Any("payload", payload),
				zap.Time("inicio", start),
			)

			err := next(ctx, payload)

			if err != nil {
				logger.Error("Task falhou",
					zap.Any("payload", payload),
					zap.Duration("duração", time.Since(start)),
					zap.Error(err),
				)
			} else {
				logger.Info("Task concluída com sucesso",
					zap.Any("payload", payload),
					zap.Duration("duração", time.Since(start)),
				)
			}

			return err
		}
	}
}
