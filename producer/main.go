package main

import (
	"encoding/json"
	"log"
	"net/http"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Pedido struct {
	ID    string  `json:"id"`
	Valor float64 `json:"valor"`
}

var ch *amqp.Channel

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal("Erro ao conectar no RabbitMQ:", err)
	}

	ch, err = conn.Channel()
	if err != nil {
		log.Fatal("Erro ao abrir canal:", err)
	}

	exchange := "pedidos.exchange"
	queue := "pedidos.criados"
	route := "pedidos.criados"

	err = ch.ExchangeDeclare(
		exchange,
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

	err = ch.QueueBind(queue, route, exchange, false, nil)
	if err != nil {
		log.Fatal("Erro ao dar bind:", err)
	}

	http.HandleFunc("/pedido", HandlePedido)

	log.Println("Produtor rodando na porta 3000...")
	http.ListenAndServe(":3000", nil)
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
		"pedidos.criados",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		http.Error(w, "Erro ao publicar mensagem", 500)
		return
	}

	w.Write([]byte(`{"status":"ok"}`))
}
