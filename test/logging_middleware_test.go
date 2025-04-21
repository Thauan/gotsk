package test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/Thauan/gotsk/internal"
)

func captureOutput(f func()) string {
	r, w, _ := os.Pipe()

	stdErr := os.Stderr
	os.Stderr = w

	out := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		out <- buf.String()
	}()

	f()

	w.Close()
	os.Stderr = stdErr

	return <-out
}

func TestLoggingMiddleware(t *testing.T) {
	logged := captureOutput(func() {
		logger := log.New(os.Stderr, "", log.LstdFlags)
		mw := internal.LoggingMiddleware(logger)
		handler := mw(func(ctx context.Context, payload interfaces.Payload) error {
			fmt.Fprintln(os.Stderr, "executando task") // escreve no stderr também
			return nil
		})

		err := handler(context.Background(), interfaces.Payload{"key": "value"})
		if err != nil {
			t.Fatalf("Handler retornou erro inesperado: %v", err)
		}
	})

	if !strings.Contains(logged, "Starting task with payload") {
		t.Errorf("esperava log de início, mas não encontrou:\n%s", logged)
	}
	if !strings.Contains(logged, "Task completed successfully") {
		t.Errorf("esperava log de finalização, mas não encontrou:\n%s", logged)
	}
	if !strings.Contains(logged, "executando task") {
		t.Errorf("esperava log do handler, mas não encontrou:\n%s", logged)
	}
}
