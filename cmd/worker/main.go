package main

import (
	"context"
	"encoding/json"
	"go-lobby/config"
	"go-lobby/internal/event"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatal(err)
	}
	conn, err := amqp.Dial(cfg.RabbitMQ.URL)
	if err != nil {
		log.Fatal(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
	}
	if err := ch.ExchangeDeclare(
		cfg.RabbitMQ.Exchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Fatal(err)
	}
	if cfg.RabbitMQ.MatchResultQueue != "" {
		if _, err := ch.QueueDeclare(
			cfg.RabbitMQ.MatchResultQueue,
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			log.Fatal(err)
		}
	}

	if err := ch.QueueBind(
		cfg.RabbitMQ.MatchResultQueue,
		"match.result.finished",
		cfg.RabbitMQ.Exchange,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		log.Fatal(err)
	}
	msgs, err := ch.Consume(
		cfg.RabbitMQ.MatchResultQueue,
		"go-lobby-rank-worker",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("worker start")
	ctx := context.Background()
	_ = ctx
	for msg := range msgs {
		var evt event.MatchResultFinishedEvent
		if err := json.Unmarshal(msg.Body, &evt); err != nil {
			log.Printf("格式有误: %v body: %s", err, string(msg.Body))
			_ = msg.Nack(false, false)
			continue
		}
		log.Printf("收到比赛结果时间: event_id=%s match_id=%d mode=%s win_team_no=%d",
			evt.EventID,
			evt.MatchID,
			evt.Mode,
			evt.WinTeamNo,
		)
		if err := msg.Ack(false); err != nil {
			log.Printf("ack失败 %v", err)
		}
	}

}
