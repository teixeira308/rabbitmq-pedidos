# RabbitMQ com Go – Producer & Consumer (Produção)

Exemplo **realista** de uso do RabbitMQ com Go, demonstrando padrões comuns de **mensageria em produção**:
retry com TTL, Dead Letter Queue (DLQ), QoS, shutdown gracioso e resiliência de conexão.

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

### 🔹 Producer

- **API HTTP** simples na porta `3000`.
- Recebe requisições `POST /pedido` com um JSON (`{"id": "...", "valor": ...}`).
- Publica a mensagem na exchange `pedidos.exchange`.
- **Não conhece** regras de retry ou DLQ (responsabilidade do consumer).
- Utiliza `sync.Mutex` para garantir que publicações concorrentes sejam seguras.

### 🔹 Consumer

- Conecta-se ao RabbitMQ com **lógica de retry** em caso de falha inicial.
- **Declara toda a topologia** (exchange, filas, bindings), garantindo que o sistema inicie de forma idempotente.
- Consome mensagens da fila principal (`pedidos.criados`) de forma concorrente com goroutines.
- Implementa:
  - **QoS (prefetch = 1):** Processa uma mensagem por vez por goroutine, evitando sobrecarga.
  - **Retry com TTL:** Mensagens com erro são enviadas para uma fila de retry.
  - **Dead Letter Queue (DLQ):** Após o número máximo de tentativas, a mensagem vai para a DLQ.
  - **Contagem de tentativas customizada:** Usa um header `x-retry-count` para controlar o ciclo de vida da mensagem.
  - **Shutdown Gracioso:** Ao receber `SIGINT` ou `SIGTERM`, para de consumir novas mensagens e espera o processamento das atuais finalizar.

---

## 🧠 Arquitetura de Mensagens

A topologia é declarada pelo Consumer para garantir que todas as filas e exchanges existam antes do processamento começar.

### Exchange

- **Tipo:** `direct`
- **Nome:** `pedidos.exchange`

### Filas

| Fila             | Finalidade                               | Configuração Notável                                     |
| -----------------|---------------------------------------- | -------------------------------------------------------- |
| `pedidos.criados`| Fila principal para novos pedidos        | -                                                        |
| `pedidos.retry`  | Fila de espera para novas tentativas     | `x-message-ttl`, `x-dead-letter-exchange`                |
| `pedidos.dlq`    | Mensagens que falharam definitivamente   | -                                                        |

### Fluxo Detalhado

1.  O **Producer** recebe um pedido via HTTP e publica a mensagem na exchange `pedidos.exchange` com a routing key `pedidos.criados`.
2.  A exchange direciona a mensagem para a fila `pedidos.criados`.
3.  O **Consumer** busca uma mensagem da fila.
4.  **Caminho Feliz:** A mensagem é processada com sucesso. O consumer envia um `ACK` manual.
5.  **Erro de Processamento (ex: valor > 1000):**
    a. O Consumer verifica o header `x-retry-count`.
    b. Se `x-retry-count < maxRetries`, o consumer incrementa o contador e publica a **mesma mensagem** na exchange com a routing key `pedidos.retry`. A mensagem original é `ACK`.
    c. A mensagem aguarda na fila `pedidos.retry` pelo tempo definido no `x-message-ttl`.
    d. Após o TTL expirar, a fila de retry, configurada com um `dead-letter-exchange`, reenvia a mensagem para a exchange `pedidos.exchange` com a routing key original (`pedidos.criados`). O fluxo recomeça no passo 2.
6.  **Falha Definitiva:**
    a. Se `x-retry-count >= maxRetries`, o consumer publica a mensagem na exchange com a routing key `pedidos.dlq` para análise posterior.
    b. Mensagens com JSON inválido também são enviadas diretamente para a `pedidos.dlq`.

---

## 🔁 Retry e Dead Letter

O padrão de retry implementado é robusto e comum em ambientes de produção.

- **Mecanismo:** Uma fila de `retry` dedicada com `x-message-ttl` e `x-dead-letter-exchange`.
- **Controle de Tentativas:** A contagem é feita via um header customizado (`x-retry-count`) que o próprio consumer gerencia. Essa abordagem é simples e explícita, controlando o fluxo diretamente na aplicação.

> **Nota de Design:** Ao contrário do que se pode inferir pelo uso de DLX para o retry, o header `x-death` não é usado aqui para a contagem. O `x-death` informa sobre as "mortes" da mensagem (como expiração de TTL), mas usar um header customizado para a lógica de negócio (`x-retry-count`) torna o controle de tentativas mais claro e desacoplado dos detalhes de implementação do broker.

---

## ✨ Padrões de Produção Implementados

- **QoS (Qualidade de Serviço):** `ch.Qos(1, 0, false)` garante que o consumer só pegue uma nova mensagem após finalizar a atual, dando previsibilidade ao processamento.
- **Shutdown Gracioso:** Captura sinais do sistema (`SIGINT`, `SIGTERM`) para finalizar o trabalho em andamento sem perda de mensagens, essencial para deploys em contêineres (Docker, Kubernetes).
- **Idempotência:** Tanto Producer quanto Consumer declaram a exchange, e o Consumer declara as filas. Isso garante que a aplicação funcione corretamente mesmo que seja iniciada antes do RabbitMQ ou que as filas não existam.
- **Resiliência de Conexão:** O Consumer tenta se reconectar ao RabbitMQ em um loop caso a conexão não esteja disponível no momento da inicialização.

---

## observability Observabilidade

O `docker-compose.yml` já inclui serviços de **Prometheus** e **Grafana** para monitoramento.

A configuração padrão do Prometheus está pronta para coletar métricas diretamente do RabbitMQ, desde que o plugin `rabbitmq_prometheus` esteja habilitado no broker.

Isso cria uma base sólida para a observabilidade do sistema, que pode ser expandida com métricas customizadas no Producer e Consumer.

---

## 🚀 Como Rodar

### 1️⃣ Subir a Infraestrutura

```bash
# Habilita o plugin de métricas do RabbitMQ e sobe os serviços
docker compose run --rm rabbitmq rabbitmq-plugins enable --offline rabbitmq_prometheus
docker compose up -d
```

Serviços disponíveis:

- **RabbitMQ (AMQP):** `amqp://guest:guest@localhost:5672`
- **RabbitMQ (Management):** `http://localhost:15672`
- **Prometheus:** `http://localhost:9090`
- **Grafana:** `http://localhost:3001`

### 2️⃣ Rodar o Consumer

```bash
cd consumer
go run main.go
```

### 3️⃣ Rodar o Producer

```bash
cd producer
go run main.go
```

### 4️⃣ Enviar um Pedido para Teste

Use o `curl` para simular o envio de um pedido.

**Pedido com Sucesso (Valor < 1000):**
```bash
curl -X POST http://localhost:3000/pedido \
  -H "Content-Type: application/json" \
  -d '{"id":"1","valor":100}'
```

**Pedido que Falha (Valor > 1000):**
Este vai acionar o ciclo de retry e, eventualmente, a DLQ.
```bash
curl -X POST http://localhost:3000/pedido \
  -H "Content-Type: application/json" \
  -d '{"id":"123","valor":1500}'
```

---

## 🎯 Objetivo do Projeto

Demonstrar:

- Uso correto e idiomático do RabbitMQ com Go.
- Padrões reais de mensageria (retry, dlq, qos).
- Código limpo, resiliente e operacional.
- Uma base pronta para crescer com observabilidade e tracing.