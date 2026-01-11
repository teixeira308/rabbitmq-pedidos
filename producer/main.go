package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Pedido struct {
	ID    string  `json:"id"`
	Valor float64 `json:"valor"`
}

var ch *amqp.Channel

func main() {
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		log.Fatal("RABBITMQ_URL não definida")
	}

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Fatal("Erro ao conectar no RabbitMQ:", err)
	}

	ch, err = conn.Channel()
	if err != nil {
		log.Fatal("Erro ao abrir canal:", err)
	}

	// ✅ Exchange pode existir ou não — não dá erro
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

	http.HandleFunc("/pedido", HandlePedido)

	log.Println("🚀 Producer rodando na porta 3000")
	log.Fatal(http.ListenAndServe(":3000", nil))
}

func HandlePedido(w http.ResponseWriter, r *http.Request) {
	var p Pedido

	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(p)

	err := ch.Publish(
		"pedidos.exchange",
		"pedidos.criados", // routing key
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
	w.Write([]byte(`{"status":"ok"}`))
}
