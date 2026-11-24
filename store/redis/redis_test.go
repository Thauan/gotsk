package redis

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/stretchr/testify/require"
)

// Teste de integração com Redis. Roda só com REDIS_ADDR definido, por exemplo
// localhost:6379. Sem a variável, é pulado.
func TestAckAfterStatusChangeLeavesNoPending(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR não definido")
	}
	ctx := context.Background()
	key := fmt.Sprintf("gotsk-it-%d", time.Now().UnixNano())
	s := New(addr, "", 0, key)
	t.Cleanup(func() { s.client.Del(ctx, s.queueKey, s.pendingKey) })

	require.NoError(t, s.Push(interfaces.Task{ID: "t1", Name: "job", Status: "queued"}))
	task, err := s.Pop()
	require.NoError(t, err)

	// O worker muda o status antes de confirmar: a confirmação precisa
	// funcionar mesmo assim.
	task.Status = "completed"
	require.NoError(t, s.Ack(task))

	n, err := s.client.HLen(ctx, s.pendingKey).Result()
	require.NoError(t, err)
	require.Equal(t, int64(0), n, "task confirmada não pode continuar pendente")
}
