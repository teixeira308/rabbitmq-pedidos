package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Pedido struct {
	ID    string  `json:"id"`
	Valor float64 `json:"valor"`
}

var (
	ch           *amqp.Channel
	publishMutex sync.Mutex
)

/*
====================
METRICS
====================
*/

var (
	producerRunning = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_producer_running",
		Help: "Indica se o producer está rodando",
	})

	httpRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_http_requests_total",
		Help: "Total de requisições HTTP recebidas",
	})

	publishSuccess = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_publish_success_total",
		Help: "Total de mensagens publicadas com sucesso",
	})

	publishError = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "app_publish_error_total",
		Help: "Total de erros ao publicar mensagens",
	})

	httpRequestDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "app_http_request_seconds",
		Help:    "Duração das requisições HTTP",
		Buckets: prometheus.DefBuckets,
	})
)

func init() {
	prometheus.MustRegister(
		producerRunning,
		httpRequests,
		publishSuccess,
		publishError,
		httpRequestDuration,
	)
}

func main() {
	// ====================
	// METRICS SERVER
	// ====================
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("📊 Métricas expostas em :9092/metrics")
		log.Fatal(http.ListenAndServe(":9092", nil))
	}()

	producerRunning.Set(1)
	defer producerRunning.Set(0)

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		log.Fatal("RABBITMQ_URL não definida")
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatal("Erro ao conectar no RabbitMQ:", err)
	}
	defer conn.Close()

	ch, err = conn.Channel()
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

	http.HandleFunc("/pedido", handlePedido)

	log.Println("🚀 Producer rodando na porta 3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func handlePedido(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	httpRequests.Inc()
	defer func() {
		httpRequestDuration.Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var p Pedido
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "JSON inválido", http.StatusBadRequest)
		return
	}

	body, err := json.Marshal(p)
	if err != nil {
		http.Error(w, "Erro ao serializar pedido", http.StatusInternalServerError)
		return
	}

	publishMutex.Lock()
	defer publishMutex.Unlock()

	err = ch.Publish(
		"pedidos.exchange",
		"pedidos.criados",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		publishError.Inc()
		http.Error(w, "Erro ao publicar mensagem", http.StatusInternalServerError)
		return
	}

	publishSuccess.Inc()
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"pedido enviado"}`))
}
