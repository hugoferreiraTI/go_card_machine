# Card Machine

A project built in **Go** to simulate the behavior and backend flow of a payment card machine.

The project is being developed incrementally, starting with the **PIX** payment flow and the **BR Code** standard for dynamic payment payload generation.

The goal is not only to generate a QR Code, but to explore the different components involved in a payment transaction, including:

- Transaction persistence;
- Payment states;
- Concurrent processing;
- Timeouts;
- Event-driven communication;
- Payment receipts;
- Messaging with Apache Kafka;
- Future communication between simulated payment services.

The project is intended to work as a practical backend development laboratory focused on payment systems.

---

## Technologies

- Go
- Gin
- SQLite
- Apache Kafka
- Confluent Kafka Go Client
- BR Code
- go-qrcode
- UUID
- Docker

---

# Current Status

The first complete PIX payment lifecycle is now working.

The application can currently:

- Configure the Card Machine;
- Create a new sale;
- Validate monetary values;
- Persist sales in SQLite;
- Generate a dynamic PIX BR Code payload;
- Generate a PNG QR Code;
- Keep the sale waiting for a payment asynchronously;
- Receive payment attempts;
- Approve valid payments;
- Reject invalid payment attempts;
- Keep a sale active after a rejected attempt;
- Automatically expire unpaid sales after a timeout;
- Store the approval timestamp only when a payment is successfully approved;
- Publish transaction events through Apache Kafka;
- Consume Kafka events;
- Generate a PIX payment receipt after approval.

The current transaction states are:

```text
PENDING
APPROVED
REPROVED
EXPIRED
```

---

# Current Payment Flow

The first end-to-end flow is complete.

```text
Card Machine Configuration
           ↓
      POST /Sale
           ↓
     Validate Amount
           ↓
    Create PENDING Sale
           ↓
      Save in SQLite
           ↓
   Retrieve Sale ID
           ↓
 Build Dynamic BR Code
           ↓
    Generate QR Code
           ↓
 Publish PENDING Event
           ↓
          Kafka
           ↓
 Wait for Payment Attempt
           ↓
      POST /Pay
           ↓
   Validate Payment
      ↙          ↘
 Incorrect      Correct
 Amount          Amount
    ↓               ↓
REPROVED         APPROVED
    ↓               ↓
Kafka Event      Save Status
    ↓               ↓
Sale remains    Save Approval Time
 PENDING            ↓
                    Kafka Event
                        ↓
                 Generate Receipt
```

If no successful payment is received within the configured timeout:

```text
PENDING
   ↓
Timeout
   ↓
EXPIRED
   ↓
Persist Status
   ↓
Kafka Event
```

---

# Transaction Lifecycle

A sale starts with:

```text
PENDING
```

From there, different events may occur.

A successful payment follows:

```text
PENDING
   ↓
APPROVED
```

An incorrect payment attempt follows:

```text
PENDING
   ↓
REPROVED
   ↓
PENDING
```

A rejected payment attempt does not finish the sale.

The Card Machine continues waiting for another payment attempt until the payment is approved or the sale expires.

If the payment timeout is reached:

```text
PENDING
   ↓
EXPIRED
```

This allows the sale lifecycle to represent the difference between a rejected **payment attempt** and the final state of the **sale itself**.

---

# Go Concurrency

The payment flow uses Go concurrency features to keep the Card Machine waiting for payment without blocking the HTTP server.

The current implementation explores:

- Goroutines;
- Channels;
- `select`;
- Timers;
- Asynchronous payment processing.

Conceptually:

```text
Create Sale
    ↓
Start WaitPayment goroutine
    ↓
Payment Channel
   ↙     ↘
Payment  Timer
   ↓       ↓
Process  EXPIRED
```

A `PaymentAttempt` carries the payment information and a response channel used to return the result of the attempt.

This allows the HTTP payment request and the sale lifecycle to communicate without tightly coupling both flows.

---

# PIX Payment

PIX is currently the first implemented payment method.

A payment attempt contains information such as:

```json
{
  "amount": "10.50",
  "cpf": "12345678901",
  "name": "Hugo Ferreira",
  "bank": "Nubank"
}
```

The application validates the payment against the currently active sale.

If the amount matches the expected sale amount:

```text
PENDING
   ↓
APPROVED
```

If the amount is incorrect:

```text
PENDING
   ↓
REPROVED
```

The rejected attempt is reported, but the sale remains available for another payment attempt.

---

# BR Code

The QR Code itself is only the visual representation of the PIX payment data.

Before generating the QR Code, the application builds a **BR Code payload** containing information related to the transaction.

Examples include:

- Amount;
- Merchant name;
- Merchant city;
- Dynamic payment location;
- Transaction-related information.

The simplified generation flow is:

```text
Sale
 ↓
DynamicQRParams
 ↓
BR Code Payload
 ↓
QR Code
```

The generated payment location includes the Sale ID.

Conceptual example:

```text
Sale ID: 42
     ↓
pix.com.br/caminho/42
     ↓
BR Code
     ↓
qr.png
```

At the current stage, this project does not communicate with the real PIX infrastructure.

The QR Code represents a simulated payment flow and cannot be processed by a real financial institution.

---

# QR Code Generation

After a sale is created:

1. The sale is persisted;
2. SQLite generates the Sale ID;
3. The ID is recovered by the application;
4. The ID is included in the dynamic payment location;
5. The BR Code payload is generated;
6. The BR Code payload is converted into a PNG QR Code;
7. The sale enters the payment waiting flow.

Example:

```text
Sale
 ↓
SQLite
 ↓
Sale ID
 ↓
Dynamic Location
 ↓
BR Code
 ↓
QR Code
```

---

# Database

The project currently uses **SQLite**.

SQLite was chosen because the application is still simulating a local Card Machine.

At this stage, introducing a separate database server would add infrastructure complexity without providing significant benefits to the current Card Machine service.

SQLite is lightweight, portable, easy to distribute with the application, and well suited for:

- Local applications;
- Embedded systems;
- Prototypes;
- Development environments.

---

## Card Machine

The Card Machine table stores information used during payment processing and receipt generation.

Examples include:

- Merchant name;
- Store name;
- CPF/CNPJ;
- City;
- State;
- Financial institution information;
- Machine-related information.

A Card Machine must be configured before creating a sale.

---

## Sales

Sales currently store information such as:

```text
ID
Sale time
Approval time
Amount
Status
```

The approval timestamp is only persisted when the payment reaches:

```text
APPROVED
```

For example:

```text
PENDING
time_approve = NULL

EXPIRED
time_approve = NULL

APPROVED
time_approve = <approval timestamp>
```

This prevents a failed or expired transaction from receiving an invalid approval timestamp.

---

# Monetary Values

The API receives monetary values as strings.

Example:

```text
"25.90"
```

This avoids relying on floating-point values for financial calculations.

The amount is converted into integer components before being persisted and is reconstructed when required by payment-related operations such as BR Code generation.

Example:

```text
"25.90"
   ↓
integer representation
   ↓
"25.90"
```

A future refactor will consolidate monetary values into a single integer representing the smallest currency unit.

Example:

```text
R$ 25.90
   ↓
2590 cents
```

---

# Apache Kafka

Apache Kafka has been introduced into the project as the first messaging layer.

The integration is still at an introductory stage and will evolve as the project is separated into multiple payment services.

The Card Machine currently contains both:

```text
Kafka Producer
Kafka Consumer
```

The producer publishes payment-related events, while the consumer is capable of consuming events from Kafka.

Transaction events currently include states such as:

```text
PENDING
APPROVED
REPROVED
EXPIRED
```

Example event:

```json
{
  "SaleId": 42,
  "value": "10.50",
  "status": "APPROVED"
}
```

The Sale ID is currently used as the Kafka message key.

Conceptually:

```text
Sale 42
   ↓
PENDING
   ↓
Kafka

Sale 42
   ↓
APPROVED
   ↓
Kafka
```

Using the same Sale ID as the message key allows events related to the same sale to follow the same partitioning strategy and preserve ordering within that partition.

The current Kafka implementation is intentionally simple.

Future iterations will use Kafka for communication between independent payment services.

---

# Payment Receipt

When a PIX payment is approved, the Card Machine generates a transaction receipt.

The receipt currently contains information such as:

```text
Payment value
Approval date
Sale ID
Status

Payer
- Name
- CPF
- Institution

Receiver
- Name
- CPF/CNPJ
- Institution
```

Example:

```text
--------------------------------
        COMPROVANTE PIX
           APROVADO
--------------------------------

Valor: R$ 10.50

ID da venda: 42
Status: APPROVED

PAGADOR
Nome: Example Payer
CPF: ***********
Instituição: Example Bank

RECEBEDOR
Nome: Example Merchant
CPF/CNPJ: ***********
Instituição: Example Bank

--------------------------------
Pagamento realizado via Pix
--------------------------------
```

The current receipt is generated locally as a text file.

---

# API

## 1. Configure Card Machine

Before creating a sale, the Card Machine must be initialized.

```http
POST /card_machine
```

Example request:

```json
{
  "person_name_storage": "Hugo Ferreira",
  "storage_name": "Test Store",
  "person_cpf": "12345678901",
  "city": "Rio de Janeiro",
  "state": "RJ"
}
```

The Card Machine configuration provides the merchant information required by later payment operations.

---

## 2. Create Sale

```http
POST /Sale
```

Example request:

```json
{
  "amount": "25.90"
}
```

The amount is received as a string and validated before the sale is persisted.

Invalid example:

```json
{
  "amount": "1ssss"
}
```

Example error response:

```json
{
  "error": "invalid amount",
  "result": ""
}
```

Only one active `PENDING` sale is currently allowed at a time.

---

## 3. Pay Sale

```http
POST /Pay
```

Example request:

```json
{
  "amount": "25.90",
  "cpf": "12345678901",
  "name": "Example Payer",
  "bank": "Example Bank"
}
```

The payment attempt is sent internally to the currently active sale.

If the value matches:

```text
APPROVED
```

If the value does not match:

```text
REPROVED
```

If the sale no longer exists or has already expired, the payment is rejected.

---

# Running the Project

Clone the repository:

```bash
git clone https://github.com/hugoferreiraTI/go_card_machine
```

Enter the project directory:

```bash
cd go_card_machine
```

Install dependencies:

```bash
go mod tidy
```

The project currently uses Kafka locally, so a Kafka broker must be available before testing messaging-related functionality.

After starting the required infrastructure, run the application:

```bash
go run ./cmd
```

By default, the HTTP API runs on:

```text
http://localhost:8080
```

---

# Important: Card Machine Initialization

Before creating a sale, the Card Machine must be configured.

The expected testing sequence is:

```text
POST /card_machine
       ↓
POST /Sale
       ↓
POST /Pay
```

The sale creation flow depends on merchant information stored during Card Machine initialization.

---

# Tests

Run all tests with:

```bash
go test -v ./...
```

Run a specific test with:

```bash
go test -v -run TestHttPostSale ./...
```

The project includes tests around areas such as:

- Database connection;
- Table creation;
- Sale persistence;
- Value insertion;
- HTTP sale creation;
- Payment-related behavior.

The test suite will continue to grow as new payment methods and distributed services are introduced.

---

# First Development Cycle

The first major development cycle of the project is now complete.

It includes:

```text
Card Machine Initialization
          ↓
Sale Creation
          ↓
SQLite Persistence
          ↓
Dynamic PIX QR Code
          ↓
PENDING
          ↓
Kafka Event
          ↓
Asynchronous Payment Waiting
          ↓
Payment Attempt
          ↓
APPROVED / REPROVED / EXPIRED
          ↓
Persistence
          ↓
Kafka Event
          ↓
Payment Receipt
```

This represents the first complete transaction lifecycle implemented by the project.

---

# Next Steps

The next major development stage is adding **card payments**.

The project will begin exploring:

- Credit card payments;
- Debit card payments;
- Card-related transaction data;
- Different payment processing flows;
- Additional transaction validation;
- Payment authorization simulation.

The objective is to avoid coupling the entire application to PIX and gradually introduce a more generic payment architecture.

Conceptually:

```text
Sale
 ↓
Payment Method
 ├── PIX
 ├── Credit Card
 └── Debit Card
```

---

# Future PSP Architecture

A later stage of the project will introduce a separate simulated **PSP — Payment Service Provider**.

The long-term architecture is expected to evolve toward something similar to:

```text
Card Machine
     │
     │ payment.created
     ▼
   Kafka
     │
     ▼
    PSP
     │
     │ Process Payment
     ▼
Simulated Banking / Payment Infrastructure
     │
     ▼
    PSP
     │
     │ payment.result
     ▼
   Kafka
     │
     ▼
Card Machine
```

Both services may contain producers and consumers.

For example:

```text
Card Machine
├── Kafka Producer
└── Kafka Consumer

PSP
├── Kafka Consumer
└── Kafka Producer
```

This architecture will allow the project to explore real distributed-system concerns naturally.

---

# Future Engineering Topics

As the project grows, future iterations will explore:

- Credit and debit card flows;
- PSP simulation;
- Service-to-service communication;
- Kafka-based asynchronous messaging;
- Idempotency;
- Payment IDs;
- Transaction IDs;
- Event IDs;
- Retries;
- Duplicate event handling;
- Consumer idempotency;
- API Gateway concepts;
- Distributed state;
- Failure recovery;
- Event ordering;
- Reconciliation;
- Improved transaction persistence;
- Monetary values stored entirely in minor units;
- More complete automated tests.

One important future problem will be ensuring that retries of the same payment operation cannot generate duplicate financial effects.

Conceptually:

```text
Same Payment
     ↓
Request
     ↓
Failure / Retry
     ↓
Same Idempotency Key
     ↓
Single Financial Effect
```

---

# Frontend

The current focus of the project is backend development.

A frontend may be introduced later to provide a visual representation of the Card Machine and make it easier to demonstrate the complete transaction lifecycle.

Possible interfaces include:

- Sale creation;
- QR Code display;
- Payment status updates;
- Transaction history;
- Payment receipt visualization.

The frontend is not currently the primary technical focus of the project.

---

# Goal

The main purpose of this project is to use a real-world payment problem as a practical backend development laboratory with Go.

Rather than simply reproducing the visual interface of a card machine, the project focuses on understanding and implementing concepts behind payment processing, including:

- Transaction lifecycle;
- State transitions;
- Persistence;
- Concurrency;
- Messaging;
- Failure handling;
- Payment communication;
- Distributed architecture.

The project will continue evolving incrementally as new payment methods and services are implemented.