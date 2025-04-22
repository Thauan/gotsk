package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Thauan/gotsk/interfaces"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSStore struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSStore(client *sqs.Client, queueURL string) *SQSStore {
	return &SQSStore{
		client:   client,
		queueURL: queueURL,
	}
}

func (s *SQSStore) Push(task interfaces.Task) error {
	data, err := json.Marshal(struct {
		Name    string
		Payload interfaces.Payload
	}{task.Name, task.Payload})
	if err != nil {
		return err
	}

	_, err = s.client.SendMessage(context.TODO(), &sqs.SendMessageInput{
		QueueUrl:    &s.queueURL,
		MessageBody: aws.String(string(data)),
	})
	return err
}

func (s *SQSStore) Pop() (interfaces.Task, error) {
	out, err := s.client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
		QueueUrl:            &s.queueURL,
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     10,
	})
	if err != nil || len(out.Messages) == 0 {
		return interfaces.Task{}, errors.New("no messages received")
	}

	msg := out.Messages[0]
	var task interfaces.Task
	if err := json.Unmarshal([]byte(*msg.Body), &task); err != nil {
		return interfaces.Task{}, err
	}
	task.ReceiptHandle = *msg.ReceiptHandle

	return task, nil
}

func (s *SQSStore) Ack(task interfaces.Task) error {
	_, err := s.client.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
		QueueUrl:      &s.queueURL,
		ReceiptHandle: &task.ReceiptHandle,
	})
	return err
}
