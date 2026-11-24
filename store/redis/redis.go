// Package redis implementa interfaces.TaskStore sobre Redis.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Thauan/gotsk/interfaces"
	goredis "github.com/redis/go-redis/v9"
)

// Store is a TaskStore backed by Redis lists.
type Store struct {
	client     *goredis.Client
	queueKey   string
	pendingKey string
}

// New cria um store Redis. baseKey é o prefixo das chaves da fila.
func New(addr string, password string, db int, baseKey string) *Store {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &Store{
		client:     rdb,
		queueKey:   fmt.Sprintf("%s:queue", baseKey),
		pendingKey: fmt.Sprintf("%s:pending", baseKey),
	}
}

// Push adds a task to the queue.
func (s *Store) Push(task interfaces.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	return s.client.LPush(context.Background(), s.queueKey, data).Err()
}

// Pop takes the next task from the queue.
func (s *Store) Pop() (interfaces.Task, error) {
	ctx := context.Background()
	data, err := s.client.RPop(ctx, s.queueKey).Result()
	if err == goredis.Nil {
		return interfaces.Task{}, errors.New("no tasks available")
	}
	if err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to pop task: %w", err)
	}

	var task interfaces.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	// Pendentes são indexados pelo ID da task. Confirmar pelo conteúdo JSON
	// falharia quando o worker altera a task (ex.: status) antes do Ack.
	if err := s.client.HSet(ctx, s.pendingKey, task.ID, data).Err(); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to move to pending: %w", err)
	}

	return task, nil
}

// Ack removes a popped task from the pending set.
func (s *Store) Ack(task interfaces.Task) error {
	removed, err := s.client.HDel(context.Background(), s.pendingKey, task.ID).Result()
	if err != nil {
		return fmt.Errorf("failed to acknowledge task: %w", err)
	}
	if removed == 0 {
		return fmt.Errorf("task %s not found in pending", task.ID)
	}
	return nil
}

// Size devolve quantas tasks estão na fila.
func (s *Store) Size(ctx context.Context) (int, error) {
	n, err := s.client.LLen(ctx, s.queueKey).Result()
	return int(n), err
}
