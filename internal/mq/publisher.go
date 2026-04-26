package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"go-lobby/config"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
}

func NewPublisher(cfg config.RabbitMQConfig) (*Publisher, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq连接失败:%w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq channel创建失败:%w", err)
	}
	if err := ch.ExchangeDeclare(
		cfg.Exchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("交换声明失败:%w", err)
	}
	if cfg.MatchResultQueue != "" {
		if _, err := ch.QueueDeclare(
			cfg.MatchResultQueue,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, fmt.Errorf("队列声明失败:%w", err)
		}
	}

	if err := ch.QueueBind(
		cfg.MatchResultQueue,
		"match.result.finished",
		cfg.Exchange,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("队列绑定失败:%w", err)
	}
	return &Publisher{
		conn:     conn,
		channel:  ch,
		exchange: cfg.Exchange,
	}, nil
}
func (p *Publisher) PublishJSON(ctx context.Context, routingKey string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("json格式有误: %w", err)
	}
	err = p.channel.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("发布失败: %w", err)
	}
	return nil
}

func (p *Publisher) Close() error {
	if p == nil {
		return nil
	}
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	return nil
}
