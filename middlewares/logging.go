package middlewares

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/fatih/color"
)

func LoggingMiddleware(logger *log.Logger) interfaces.Middleware {
	successColor := color.New(color.FgGreen, color.Bold)
	errorColor := color.New(color.FgRed, color.Bold)
	warningColor := color.New(color.FgYellow, color.Bold)
	infoColor := color.New(color.FgCyan, color.Bold)
	separatorColor := color.New(color.FgMagenta, color.Bold)

	return func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, payload interfaces.Payload) error {
			start := time.Now()
			taskID := "unknown"
			if t, ok := ctx.Value("task").(interfaces.Task); ok {
				taskID = t.ID
			}

			printBox(separatorColor, "🚀 INICIANDO TASK", 50)
			logger.Printf("│ %-15s: %s", "Task ID", infoColor.Sprint(taskID))
			logger.Printf("│ %-15s: %s", "Hora de Início", infoColor.Sprint(start.Format("2006-01-02 15:04:05.000")))
			logPayload(logger, payload)
			printSeparator(separatorColor, 50)

			err := next(ctx, payload)

			duration := time.Since(start)
			printBox(separatorColor, "📊 RESUMO DA TASK", 50)
			logger.Printf("│ %-15s: %s", "Task ID", infoColor.Sprint(taskID))
			logger.Printf("│ %-15s: %s", "Duração", formatDuration(duration))

			if err != nil {
				logger.Printf("│ %-15s: %s", "Status", errorColor.Sprint("❌ Task falhou"))
				logger.Printf("│ %-15s: %v", "Erro", errorColor.Sprint(err))
			} else {
				logger.Printf("│ %-15s: %s", "Status", successColor.Sprint("✅ Task finalizada"))
			}

			if duration > time.Second {
				logger.Printf("│ %-15s: %s", "Performance", warningColor.Sprint("⚠️ LENTO"))
			} else if duration > 500*time.Millisecond {
				logger.Printf("│ %-15s: %s", "Performance", warningColor.Sprint("⏱️  MÉDIO"))
			} else {
				logger.Printf("│ %-15s: %s", "Performance", successColor.Sprint("⚡ RÁPIDO"))
			}

			printSeparator(separatorColor, 50)
			logger.Println()

			return err
		}
	}
}

func printBox(c *color.Color, text string, width int) {
	boxTop := "┌" + strings.Repeat("─", width-2) + "┐"
	boxBottom := "└" + strings.Repeat("─", width-2) + "┘"

	textLen := utf8.RuneCountInString(text)
	padding := (width - textLen - 4) / 2
	leftPadding := strings.Repeat(" ", padding)
	rightPadding := strings.Repeat(" ", width-textLen-padding-4)

	c.Println(boxTop)
	c.Printf("│%s%s%s│\n", leftPadding, text, rightPadding)
	c.Println(boxBottom)
}

func printSeparator(c *color.Color, width int) {
	c.Println("├" + strings.Repeat("─", width-2) + "┤")
}

func logPayload(logger *log.Logger, payload interfaces.Payload) {
	if len(payload) == 0 {
		logger.Printf("│ %-15s: %s", "Payload", "vazio")
		return
	}

	logger.Printf("│ %-15s:", "Payload")
	for k, v := range payload {
		valueStr := fmt.Sprintf("%v", v)
		if len(valueStr) > 50 {
			valueStr = valueStr[:47] + "..."
		}
		logger.Printf("│   %-12s: %s", k, valueStr)
	}
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fµs", float64(d.Microseconds()))
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Milliseconds()))
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
