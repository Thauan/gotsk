// Package sqs implementa interfaces.TaskStore sobre Amazon SQS.
package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type Store struct {
	client   *sqs.Client
	queueURL string
	mu       sync.Mutex
	pending  map[string]string
}

// New cria um store SQS para a fila queueURL.
func New(client *sqs.Client, queueURL string) *Store {
	return &Store{
		client:   client,
		queueURL: queueURL,
		pending:  make(map[string]string),
	}
}

func (s *Store) Push(task interfaces.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	_, err = s.client.SendMessage(context.TODO(), &sqs.SendMessageInput{
		QueueUrl:    &s.queueURL,
		MessageBody: awsString(string(data)),
	})
	if err != nil {
		return fmt.Errorf("failed to send message to SQS: %w", err)
	}

	return nil
}

func (s *Store) Pop() (interfaces.Task, error) {
	resp, err := s.client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
		QueueUrl:            &s.queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     10,
	})
	if err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to receive message: %w", err)
	}

	if len(resp.Messages) == 0 {
		return interfaces.Task{}, errors.New("no tasks available")
	}

	msg := resp.Messages[0]
	var task interfaces.Task
	if err := json.Unmarshal([]byte(*msg.Body), &task); err != nil {
		return interfaces.Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	if task.ID == "" {
		return interfaces.Task{}, errors.New("task missing ID")
	}

	s.mu.Lock()
	s.pending[task.ID] = *msg.ReceiptHandle
	s.mu.Unlock()

	return task, nil
}

func (s *Store) Ack(task interfaces.Task) error {
	s.mu.Lock()
	receipt, ok := s.pending[task.ID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("receipt handle not found for task ID: %s", task.ID)
	}
	delete(s.pending, task.ID)
	s.mu.Unlock()

	_, err := s.client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
		QueueUrl:      &s.queueURL,
		ReceiptHandle: &receipt,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message from SQS: %w", err)
	}

	return nil
}

func awsString(s string) *string {
	return &s
}
