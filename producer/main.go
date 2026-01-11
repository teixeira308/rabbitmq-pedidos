package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

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

func main() {
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
		http.Error(w, "Erro ao publicar mensagem", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"pedido enviado"}`))
}
