package test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/middlewares"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestZapLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)

	logger := zap.New(core)
	defer logger.Sync()

	middleware := middlewares.ZapLoggingMiddleware(logger)

	handler := middleware(func(ctx context.Context, payload interfaces.Payload) error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})

	err := handler(context.Background(), interfaces.Payload{"key": "value"})
	if err != nil {
		t.Fatalf("Handler retornou erro inesperado: %v", err)
	}

	logs := buf.String()
	if !contains(logs, "Iniciando task") {
		t.Errorf("Log de início da task não encontrado. Logs: %s", logs)
	}
	if !contains(logs, "Task concluída com sucesso") {
		t.Errorf("Log de sucesso da task não encontrado. Logs: %s", logs)
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
