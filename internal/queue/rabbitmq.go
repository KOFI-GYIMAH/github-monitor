package queue

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/streadway/amqp"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func NewRabbitMQ(url string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	return &RabbitMQ{
		conn:    conn,
		channel: channel,
	}, nil
}

func (r *RabbitMQ) PublishSyncRequest(ctx context.Context, owner, repo string, since time.Time) error {
	queue, err := r.channel.QueueDeclare(
		"github_sync",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msg := struct {
		Owner string    `json:"owner"`
		Repo  string    `json:"repo"`
		Since time.Time `json:"since"`
	}{
		Owner: owner,
		Repo:  repo,
		Since: since,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return r.channel.Publish(
		"",
		queue.Name,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func (r *RabbitMQ) ConsumeSyncRequests(ctx context.Context, handler func(owner, repo string, since time.Time) error) error {
	queue, err := r.channel.QueueDeclare(
		"github_sync",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	msgs, err := r.channel.Consume(
		queue.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			var msg struct {
				Owner string    `json:"owner"`
				Repo  string    `json:"repo"`
				Since time.Time `json:"since"`
			}

			if err := json.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("Error decoding message: %v", err)
				continue
			}

			if err := handler(msg.Owner, msg.Repo, msg.Since); err != nil {
				log.Printf("Error handling sync request: %v", err)
			}
		}
	}()

	return nil
}

func (r *RabbitMQ) Close() error {
	if err := r.channel.Close(); err != nil {
		return err
	}
	return r.conn.Close()
}
