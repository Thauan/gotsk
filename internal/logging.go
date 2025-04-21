package internal

import (
	"context"
	"log"

	"github.com/Thauan/gotsk/interfaces"
)

func LoggingMiddleware(logger *log.Logger) interfaces.Middleware {
	return func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, payload interfaces.Payload) error {
			logger.Printf("Starting task with payload: %v", payload)
			err := next(ctx, payload)
			if err != nil {
				logger.Printf("Task failed: %v", err)
			} else {
				logger.Println("Task completed successfully")
			}
			return err
		}
	}
}
