package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Pedido struct {
	ID    string  `json:"id"`
	Valor float64 `json:"valor"`
}

const (
	exchange   = "pedidos.exchange"
	queueMain  = "pedidos.criados"
	queueRetry = "pedidos.retry"
	queueDLQ   = "pedidos.criados.dlq"
	maxRetries = 3
)

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer ch.Close()

	declareTopology(ch)

	if err := ch.Qos(1, 0, false); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	consumerTag := "pedidos-consumer"

	msgs, err := ch.Consume(
		queueMain,
		consumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	go handleShutdown(cancel, ch, consumerTag)

	log.Println("✅ Consumidor rodando")

	for {
		select {
		case <-ctx.Done():
			log.Println("⏳ Aguardando mensagens em processamento...")
			wg.Wait()
			log.Println("🛑 Shutdown completo")
			return

		case msg, ok := <-msgs:
			if !ok {
				return
			}

			wg.Add(1)
			go processMessage(ch, msg, wg)
		}
	}
}

func processMessage(ch *amqp.Channel, msg amqp.Delivery, wg *sync.WaitGroup) {
	defer wg.Done()

	var pedido Pedido
	if err := json.Unmarshal(msg.Body, &pedido); err != nil {
		log.Println("Payload inválido → DLQ")
		msg.Nack(false, false)
		return
	}

	retries := getRetryCount(msg)

	if pedido.Valor > 1000 {
		if retries >= maxRetries {
			log.Println("❌ Estourou retries → DLQ", pedido)
			msg.Nack(false, false)
			return
		}

		log.Println("🔁 Retry", retries+1)

		if err := ch.Publish(
			exchange,
			queueRetry,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        msg.Body,
			},
		); err != nil {
			msg.Nack(false, true)
			return
		}

		msg.Ack(false)
		return
	}

	log.Println("✅ Processado com sucesso:", pedido)
	msg.Ack(false)
}

func handleShutdown(cancel context.CancelFunc, ch *amqp.Channel, consumerTag string) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("⚠️ Sinal de shutdown recebido")

	cancel()

	// Para de receber novas mensagens
	ch.Cancel(consumerTag, false)

	// Tempo de graça (opcional, mas profissional)
	time.Sleep(2 * time.Second)
}

func getRetryCount(msg amqp.Delivery) int {
	deaths, ok := msg.Headers["x-death"].([]interface{})
	if !ok || len(deaths) == 0 {
		return 0
	}

	death := deaths[0].(amqp.Table)
	if count, ok := death["count"].(int64); ok {
		return int(count)
	}
	return 0
}

func declareTopology(ch *amqp.Channel) {
	ch.ExchangeDeclare(exchange, "direct", true, false, false, false, nil)

	ch.QueueDeclare(queueMain, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    exchange,
		"x-dead-letter-routing-key": queueDLQ,
	})

	ch.QueueDeclare(queueRetry, true, false, false, false, amqp.Table{
		"x-message-ttl":             int32(5000),
		"x-dead-letter-exchange":    exchange,
		"x-dead-letter-routing-key": queueMain,
	})

	ch.QueueDeclare(queueDLQ, true, false, false, false, nil)

	ch.QueueBind(queueMain, queueMain, exchange, false, nil)
	ch.QueueBind(queueRetry, queueRetry, exchange, false, nil)
	ch.QueueBind(queueDLQ, queueDLQ, exchange, false, nil)
}
