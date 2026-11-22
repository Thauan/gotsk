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

func (s *Store) Push(task interfaces.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	return s.client.LPush(context.Background(), s.queueKey, data).Err()
}

func (s *Store) Pop() (interfaces.Task, error) {
	ctx := context.Background()
	data, err := s.client.RPop(ctx, s.queueKey).Result()
	if err == goredis.Nil {
		return interfaces.Task{}, errors.New("no tasks available")
	}
	if err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to pop task: %w", err)
	}

	if err := s.client.LPush(ctx, s.pendingKey, data).Err(); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to move to pending: %w", err)
	}

	var task interfaces.Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return task, nil
}

func (s *Store) Ack(task interfaces.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task for ack: %w", err)
	}

	return s.client.LRem(context.Background(), s.pendingKey, 1, data).Err()
}
