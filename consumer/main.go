package main

import (
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Pedido struct {
	ID    string  `json:"id"`
	Valor float64 `json:"valor"`
}

func main() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal("Erro ao conectar RabbitMQ:", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatal("Erro ao abrir canal:", err)
	}

	exchange := "pedidos.exchange"

	// Fila principal com DLX configurado
	_, err = ch.QueueDeclare(
		"pedidos.criados",
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    exchange,
			"x-dead-letter-routing-key": "pedidos.criados.dlq",
		},
	)
	if err != nil {
		log.Fatal("Erro ao declarar fila principal:", err)
	}

	// Fila DLQ
	_, err = ch.QueueDeclare(
		"pedidos.criados.dlq",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal("Erro ao declarar DLQ:", err)
	}

	err = ch.QueueBind("pedidos.criados.dlq", "pedidos.criados.dlq", exchange, false, nil)
	if err != nil {
		log.Fatal("Erro ao bind DLQ:", err)
	}

	msgs, err := ch.Consume(
		"pedidos.criados",
		"",
		false,
		false,
		false,
		false,
		nil,
	)

	log.Println("Consumidor rodando...")

	for msg := range msgs {
		var pedido Pedido
		json.Unmarshal(msg.Body, &pedido)

		// Regra de negócio simulada
		if pedido.Valor > 1000 {
			log.Println("Erro: pedido acima do limite → DLQ:", pedido)
			ch.Nack(msg.DeliveryTag, false, false) // não requeue
			continue
		}

		log.Println("Processado com sucesso:", pedido)
		ch.Ack(msg.DeliveryTag, false)
	}
}
