package interfaces

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSStore struct {
	client   *sqs.Client
	queueURL string
	mu       sync.Mutex
	pending  map[string]string
}

func NewSQSStore(client *sqs.Client, queueURL string) *SQSStore {
	return &SQSStore{
		client:   client,
		queueURL: queueURL,
		pending:  make(map[string]string),
	}
}

func (s *SQSStore) Push(task Task) error {
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

func (s *SQSStore) Pop() (Task, error) {
	resp, err := s.client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
		QueueUrl:            &s.queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     10,
	})
	if err != nil {
		return Task{}, fmt.Errorf("failed to receive message: %w", err)
	}

	if len(resp.Messages) == 0 {
		return Task{}, errors.New("no tasks available")
	}

	msg := resp.Messages[0]
	var task Task
	if err := json.Unmarshal([]byte(*msg.Body), &task); err != nil {
		return Task{}, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	if task.ID == "" {
		return Task{}, errors.New("task missing ID")
	}

	s.mu.Lock()
	s.pending[task.ID] = *msg.ReceiptHandle
	s.mu.Unlock()

	return task, nil
}

func (s *SQSStore) Ack(task Task) error {
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
