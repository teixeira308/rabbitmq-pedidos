# RabbitMQ com Go – Producer & Consumer Simples

Um exemplo introdutório de um sistema de mensageria com RabbitMQ e Go, consistindo de um Producer (produtor) e um Consumer (consumidor).

---

## 📁 Estrutura do Projeto

```text
.
├── docker-compose.yml
├── producer/
│   └── main.go
└── consumer/
    └── main.go
```

### 🔹 producer

- Expõe uma API HTTP na porta `3000`.
- Aceita requisições `POST` no endpoint `/pedido`.
- Publica a mensagem recebida em uma exchange do RabbitMQ.

### 🔹 consumer

- Conecta-se ao RabbitMQ.
- Consome mensagens de uma fila.
- Imprime no console as mensagens que recebe.
- Utiliza **Auto-acknowledgement**, onde as mensagens são consideradas confirmadas assim que são entregues pelo broker.

---

## 🧠 Arquitetura de Mensagens

### Fluxo

1.  O **Producer** publica uma mensagem na exchange `pedidos.exchange` com a routing key `pedidos.criados`.
2.  A exchange direciona a mensagem para a fila `pedidos.queue`, que está associada a essa routing key.
3.  O **Consumer** recebe a mensagem da fila e a exibe no log.

### Componentes

- **Exchange:**
  - **Nome:** `pedidos.exchange`
  - **Tipo:** `direct`
- **Queue:**
  - **Nome:** `pedidos.queue`
- **Binding:**
  - A fila `pedidos.queue` é vinculada (bind) à exchange `pedidos.exchange` com a routing key `pedidos.criados`.

---

## 🚀 Como Rodar

### 1️⃣ Subir o RabbitMQ

O `docker-compose.yml` provisiona um container do RabbitMQ.

```bash
docker compose up -d
```

RabbitMQ disponível em:
- **AMQP:** `amqp://guest:guest@localhost:5672`
- **Management UI:** `http://localhost:15672`

### 2️⃣ Rodar o Consumer

Em um terminal, navegue até a pasta do consumer e execute:

```bash
cd consumer
go run main.go
```

O consumer ficará aguardando por mensagens.

### 3️⃣ Rodar o Producer

Em outro terminal, inicie o producer:

```bash
cd producer
go run main.go
```

O producer estará pronto para receber requisições HTTP.

### 4️⃣ Enviar um Pedido

Use o `curl` para enviar um pedido ao producer, que o encaminhará como uma mensagem:

```bash
curl -X POST http://localhost:3000/pedido \
  -H "Content-Type: application/json" \
  -d '{"id":"123","valor":1500}'
```

Ao executar o comando acima, você verá o log do pedido aparecer no terminal onde o **consumer** está rodando.
