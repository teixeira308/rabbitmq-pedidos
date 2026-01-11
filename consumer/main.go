package main

import (
	"log"
	"os"

	amqp "github.com/rabbitmq/amqp091-go"
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
		true,
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
			log.Println("Pedido recebido:", string(d.Body))
		}
	}()

	<-forever
}
