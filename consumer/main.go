package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

/*
====================
METRICS
====================
*/

var (
	consumerRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_consumer_running",
		Help: "Indica se o consumer está rodando",
	})

	messagesProcessed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_messages_processed_total",
		Help: "Total de mensagens processadas com sucesso",
	})

	messagesRetry = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_messages_retry_total",
		Help: "Total de mensagens enviadas para retry",
	})

	messagesDLQ = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_messages_dlq_total",
		Help: "Total de mensagens enviadas para DLQ",
	})

	messageProcessingTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "app_message_processing_seconds",
		Help:    "Tempo de processamento das mensagens",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(
		consumerRunning,
		messagesProcessed,
		messagesRetry,
		messagesDLQ,
		messageProcessingTime,
	)
}

func main() {
	// ====================
	// METRICS SERVER
	// ====================
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("📊 Métricas expostas em :9091/metrics")
		log.Fatal(http.ListenAndServe(":9091", nil))
	}()

	consumerRunning.Set(1)
	defer consumerRunning.Set(0)

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		log.Fatal("RABBITMQ_URL não definida")
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatal("Erro ao conectar no RabbitMQ:", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Erro ao abrir canal:", err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"pedidos.exchange",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao declarar exchange:", err)
	}

	q, err := ch.QueueDeclare(
		"pedidos.queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao declarar fila:", err)
	}

	err = ch.QueueBind(
		q.Name,
		"pedidos.criados",
		"pedidos.exchange",
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao bindar fila:", err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true, // auto-ack por enquanto
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao consumir mensagens:", err)
	}

	log.Println("📥 Consumer aguardando mensagens...")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			start := time.Now()

			log.Println("Pedido recebido:", string(d.Body))

			// Simulação de processamento
			time.Sleep(200 * time.Millisecond)

			messagesProcessed.Inc()
			messageProcessingTime.Observe(time.Since(start).Seconds())
		}
	}()

	<-forever
}
