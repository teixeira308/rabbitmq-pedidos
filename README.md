# RabbitMQ com Go – Producer & Consumer (Produção)

Exemplo **realista** de uso do RabbitMQ com Go, demonstrando padrões comuns de **mensageria em produção**:
retry com TTL, Dead Letter Queue (DLQ), QoS, controle de backpressure e shutdown gracioso.

Projeto pensado para **estudo sério** e **portfólio**, não apenas “hello world”.

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

* API HTTP simples
* Recebe requisições `POST /pedido`
* Publica mensagens no RabbitMQ
* Não conhece regras de retry ou DLQ (responsabilidade do consumer)

### 🔹 consumer

* Consome mensagens da fila principal
* Implementa:

  * QoS (prefetch = 1)
  * Retry com TTL
  * Dead Letter Queue (DLQ)
  * Contagem de tentativas via `x-death`
  * Shutdown gracioso (SIGINT / SIGTERM)

---

## 🧠 Arquitetura de Mensagens

### Exchange

* **Tipo:** `direct`
* **Nome:** `pedidos.exchange`

### Filas

| Fila                  | Finalidade                             |
| --------------------- | -------------------------------------- |
| `pedidos.criados`     | Fila principal                         |
| `pedidos.retry`       | Fila de retry com TTL                  |
| `pedidos.criados.dlq` | Mensagens que falharam definitivamente |

### Fluxo

1. Producer publica em `pedidos.exchange`
2. Consumer consome `pedidos.criados`
3. Em erro transitório:

   * Mensagem vai para `pedidos.retry`
   * Aguarda TTL
   * Retorna automaticamente para `pedidos.criados`
4. Após estourar o número máximo de tentativas:

   * Mensagem é enviada para a DLQ

---

## 🔁 Retry e Dead Letter

* Retry implementado com:

  * `x-message-ttl`
  * `x-dead-letter-exchange`
* Contagem de tentativas baseada em:

  * Header nativo `x-death` (padrão RabbitMQ)
* Evita headers customizados e lógica frágil

---

## ⚙️ QoS (Qualidade de Serviço)

```go
ch.Qos(1, 0, false)
```

* Garante que o consumer processe **uma mensagem por vez**
* Evita sobrecarga
* Facilita controle de falhas
* Comportamento previsível em produção

---

## 🛑 Shutdown Gracioso

O consumer:

* Captura `SIGINT` e `SIGTERM`
* Para de receber novas mensagens
* Aguarda mensagens em processamento finalizarem
* Fecha canal e conexão corretamente

Essencial para:

* Docker
* Kubernetes
* Deploys sem perda de mensagens

---

## 🚀 Como Rodar

### 1️⃣ Subir o RabbitMQ

```bash
docker compose up -d
```

RabbitMQ disponível em:

* AMQP: `amqp://guest:guest@localhost:5672`
* Management UI: `http://localhost:15672`

---

### 2️⃣ Rodar o Consumer

```bash
cd consumer
go run main.go
```

---

### 3️⃣ Rodar o Producer

```bash
cd producer
go run main.go
```

---

### 4️⃣ Enviar um Pedido

```bash
curl -X POST http://localhost:3000/pedido \
  -H "Content-Type: application/json" \
  -d '{"id":"123","valor":1500}'
```

* Valores altos simulam erro de negócio
* Gatilham retry e eventualmente DLQ

---

## 📌 Decisões de Design

* Exchange e filas são **idempotentes**
* Consumer declara a topologia (modelo comum em produção)
* Producer não conhece detalhes de retry/DLQ
* Uso de headers nativos (`x-death`)
* Código organizado para fácil evolução

---

## 🎯 Objetivo do Projeto

Demonstrar:

* Uso correto do RabbitMQ com Go
* Padrões reais de mensageria
* Código limpo, previsível e operacional
* Pronto para crescer (observabilidade, métricas, tracing)

---
