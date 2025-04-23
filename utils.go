package gotsk

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WaitForInterrupt(cancel context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	cancel()
}
