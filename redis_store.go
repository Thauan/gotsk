package gotsk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	key    string
}

func NewRedisStore(addr string, password string, db int, key string) *RedisStore {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisStore{
		client: rdb,
		key:    key,
	}
}

func (s *RedisStore) Push(task Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}
	return s.client.LPush(context.Background(), s.key, data).Err()
}

func (s *RedisStore) Pop() (Task, error) {
	data, err := s.client.RPop(context.Background(), s.key).Result()
	if err == redis.Nil {
		return Task{}, errors.New("no tasks available")
	}
	if err != nil {
		return Task{}, fmt.Errorf("failed to pop task: %w", err)
	}

	var task Task
	if err := json.Unmarshal([]byte(data), &task); err != nil {
		return Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return task, nil
}

func (s *RedisStore) Ack(task Task) error {
	return nil
}
