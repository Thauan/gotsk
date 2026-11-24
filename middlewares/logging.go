package middlewares

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Thauan/gotsk/interfaces"
)

// style aplica cores ANSI quando a saída é um terminal e NO_COLOR não está
// definido. Substitui fatih/color para não adicionar dependências ao núcleo.
type style struct{ code string }

var colorEnabled = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

func (st style) Sprint(a ...any) string {
	text := fmt.Sprint(a...)
	if !colorEnabled {
		return text
	}
	return "\x1b[" + st.code + "m" + text + "\x1b[0m"
}

func (st style) Println(a ...any) {
	fmt.Println(st.Sprint(fmt.Sprintln(a...)))
}

func (st style) Printf(format string, a ...any) {
	fmt.Print(st.Sprint(fmt.Sprintf(format, a...)))
}

// LoggingMiddleware logs the start, payload, result and duration of each task
// to logger, in a boxed format.
func LoggingMiddleware(logger *log.Logger) interfaces.Middleware {
	successColor := style{"32;1"}
	errorColor := style{"31;1"}
	warningColor := style{"33;1"}
	infoColor := style{"36;1"}
	separatorColor := style{"35;1"}

	return func(next interfaces.HandlerFunc) interfaces.HandlerFunc {
		return func(ctx context.Context, payload interfaces.Payload) error {
			start := time.Now()
			taskID := "unknown"
			if t, ok := ctx.Value("task").(interfaces.Task); ok {
				taskID = t.ID
			}

			printBox(separatorColor, "🚀 INICIANDO TASK", 50)
			logger.Printf("│ %-15s: %s", "Task ID", infoColor.Sprint(taskID))
			logger.Printf("│ %-15s: %s", "Start time", infoColor.Sprint(start.Format("2006-01-02 15:04:05.000")))
			logPayload(logger, payload)
			printSeparator(separatorColor, 50)

			err := next(ctx, payload)

			duration := time.Since(start)
			printBox(separatorColor, "📊 RESUMO DA TASK", 50)
			logger.Printf("│ %-15s: %s", "Task ID", infoColor.Sprint(taskID))
			logger.Printf("│ %-15s: %s", "Duration", formatDuration(duration))

			if err != nil {
				logger.Printf("│ %-15s: %s", "Status", errorColor.Sprint("❌ Task falhou"))
				logger.Printf("│ %-15s: %v", "Erro", errorColor.Sprint(err))
			} else {
				logger.Printf("│ %-15s: %s", "Status", successColor.Sprint("✅ Task finished"))
			}

			if duration > time.Second {
				logger.Printf("│ %-15s: %s", "Performance", warningColor.Sprint("⚠️ SLOW"))
			} else if duration > 500*time.Millisecond {
				logger.Printf("│ %-15s: %s", "Performance", warningColor.Sprint("⏱️  MEDIUM"))
			} else {
				logger.Printf("│ %-15s: %s", "Performance", successColor.Sprint("⚡ FAST"))
			}

			printSeparator(separatorColor, 50)
			logger.Println()

			return err
		}
	}
}

func printBox(c style, text string, width int) {
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

func printSeparator(c style, width int) {
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
