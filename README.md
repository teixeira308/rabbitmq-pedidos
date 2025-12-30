# RabbitMQ com Go

Exemplo prático de producer e consumer usando RabbitMQ e Go.

## Estrutura
- `producer`: API HTTP que publica mensagens
- `consumer`: Consome mensagens da fila

## Tecnologias
- Go
- RabbitMQ
- AMQP 0.9.1

## Como rodar
1. Suba o RabbitMQ
2. Rode o consumer
3. Rode o producer
4. Envie POST para `/pedido`
