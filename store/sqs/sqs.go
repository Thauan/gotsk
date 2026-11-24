// Package sqs implementa interfaces.TaskStore sobre Amazon SQS.
package sqs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Store is a TaskStore backed by an Amazon SQS queue.
// Tasks popped but not acknowledged become visible again after the queue's visibility timeout.
type Store struct {
	client   *sqs.Client
	queueURL string
}

// New cria um store SQS para a fila queueURL.
func New(client *sqs.Client, queueURL string) *Store {
	return &Store{
		client:   client,
		queueURL: queueURL,
	}
}

// Push sends a task to the queue.
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

// Pop receives the next task, waiting up to 10 seconds for one.
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

	// O recibo acompanha a task: Ack usa esse valor, e não um mapa por ID,
	// que se perderia se duas tasks com o mesmo ID estivessem em voo.
	task.ReceiptHandle = aws.ToString(msg.ReceiptHandle)
	return task, nil
}

// Ack deletes a popped task from the queue.
func (s *Store) Ack(task interfaces.Task) error {
	if task.ReceiptHandle == "" {
		return fmt.Errorf("task %s has no receipt handle: it was not popped from this queue", task.ID)
	}
	_, err := s.client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
		QueueUrl:      &s.queueURL,
		ReceiptHandle: &task.ReceiptHandle,
	})
	if err != nil {
		return fmt.Errorf("failed to delete message from SQS: %w", err)
	}
	return nil
}

func awsString(s string) *string {
	return &s
}

// Size devolve a contagem aproximada de mensagens na fila (o SQS não dá um
// número exato).
func (s *Store) Size(ctx context.Context) (int, error) {
	out, err := s.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl:       &s.queueURL,
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameApproximateNumberOfMessages},
	})
	if err != nil {
		return 0, fmt.Errorf("lendo tamanho da fila SQS: %w", err)
	}
	n, err := strconv.Atoi(out.Attributes[string(sqstypes.QueueAttributeNameApproximateNumberOfMessages)])
	return n, err
}
