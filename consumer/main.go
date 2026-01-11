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
	queueDLQ   = "pedidos.dlq"

	maxRetries = 3
	retryTTL   = 5000 // ms
)

func main() {
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		log.Fatal("RABBITMQ_URL não definida")
	}

	conn := mustConnect(amqpURL)
	defer conn.Close()

	consumerCh := mustChannel(conn)
	defer consumerCh.Close()

	publisherCh := mustChannel(conn)
	defer publisherCh.Close()

	declareTopology(consumerCh)

	if err := consumerCh.Qos(1, 0, false); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	go gracefulShutdown(cancel)

	msgs, err := consumerCh.Consume(
		queueMain,
		"pedidos-consumer",
		false, // ACK MANUAL
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("📥 Consumer rodando")

	for {
		select {
		case <-ctx.Done():
			log.Println("⏳ Finalizando processamento...")
			wg.Wait()
			log.Println("🛑 Consumer finalizado")
			return

		case msg, ok := <-msgs:
			if !ok {
				log.Println("Canal fechado")
				return
			}

			wg.Add(1)
			go processMessage(msg, consumerCh, publisherCh, wg)
		}
	}
}

func processMessage(
	msg amqp.Delivery,
	consumerCh *amqp.Channel,
	publisherCh *amqp.Channel,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	var pedido Pedido
	if err := json.Unmarshal(msg.Body, &pedido); err != nil {
		log.Println("❌ JSON inválido → DLQ")
		publish(publisherCh, queueDLQ, msg.Body, nil)
		msg.Ack(false)
		return
	}

	retries := getRetryCount(msg)

	// Simulação de erro de negócio
	if pedido.Valor > 1000 {
		if retries >= maxRetries {
			log.Printf("❌ Estourou retries → DLQ | %+v\n", pedido)
			publish(publisherCh, queueDLQ, msg.Body, nil)
			msg.Ack(false)
			return
		}

		log.Printf("🔁 Retry %d/%d | %+v\n", retries+1, maxRetries, pedido)

		headers := amqp.Table{
			"x-retry-count": retries + 1,
		}

		publish(publisherCh, queueRetry, msg.Body, headers)
		msg.Ack(false)
		return
	}

	log.Println("✅ Processado com sucesso:", pedido)
	msg.Ack(false)
}

func publish(ch *amqp.Channel, routingKey string, body []byte, headers amqp.Table) {
	err := ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
			Headers:     headers,
		},
	)
	if err != nil {
		log.Println("❌ Erro ao publicar:", err)
	}
}

func getRetryCount(msg amqp.Delivery) int {
	if v, ok := msg.Headers["x-retry-count"]; ok {
		switch t := v.(type) {
		case int32:
			return int(t)
		case int64:
			return int(t)
		}
	}
	return 0
}

func declareTopology(ch *amqp.Channel) {
	_ = ch.ExchangeDeclare(exchange, "direct", true, false, false, false, nil)

	_, _ = ch.QueueDeclare(queueMain, true, false, false, false, nil)

	_, _ = ch.QueueDeclare(
		queueRetry,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-message-ttl":             int32(retryTTL),
			"x-dead-letter-exchange":    exchange,
			"x-dead-letter-routing-key": queueMain,
		},
	)

	_, _ = ch.QueueDeclare(queueDLQ, true, false, false, false, nil)

	_ = ch.QueueBind(queueMain, queueMain, exchange, false, nil)
	_ = ch.QueueBind(queueRetry, queueRetry, exchange, false, nil)
	_ = ch.QueueBind(queueDLQ, queueDLQ, exchange, false, nil)
}

func gracefulShutdown(cancel context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	<-sig
	log.Println("⚠️ SIG recebido, iniciando shutdown...")
	cancel()
}

func mustConnect(url string) *amqp.Connection {
	for i := 1; i <= 10; i++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			log.Println("✅ Conectado ao RabbitMQ")
			return conn
		}
		log.Printf("⏳ RabbitMQ indisponível (%d/10)", i)
		time.Sleep(3 * time.Second)
	}
	log.Fatal("❌ Falha ao conectar no RabbitMQ")
	return nil
}

func mustChannel(conn *amqp.Connection) *amqp.Channel {
	ch, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	return ch
}
