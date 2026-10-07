# Card Machine

A project built in **Go** to simulate the behavior of a payment card machine.

The project is being developed in stages, starting with the **PIX** payment flow and using the **BR Code** standard to generate the payment payload, which is later converted into a QR Code.

The goal is not only to generate a QR Code, but to understand and implement the different steps involved in a payment transaction, including persistence, transaction states, concurrency, and later communication between simulated PSP services.

## Technologies

- Go
- Gin
- SQLite
- BR Code
- go-qrcode
- UUID

## Current Status

The first payment flow is already working.

The application can currently:

- Configure the card machine data;
- Receive a sale amount through the API;
- Validate the received amount;
- Create a sale with `PENDING` status;
- Persist the sale in SQLite;
- Retrieve the generated sale ID;
- Use the ID to build the dynamic payment location;
- Generate a payload following the BR Code standard;
- Generate a PNG QR Code containing the payload.

## Current Flow

```text
Card Machine configuration
            ↓
       POST /Sale
            ↓
      Receive amount
            ↓
      Validate amount
            ↓
    Create PENDING Sale
            ↓
      Save in SQLite
            ↓
   Retrieve generated ID
            ↓
 Build payment location
            ↓
 Generate BR Code payload
            ↓
      Generate QR Code
```

## BR Code

The QR Code itself is only the visual representation of the payment data.

Before generating the QR Code, the application builds a **BR Code payload** containing information related to the payment, such as:

- Amount;
- Merchant name;
- Merchant city;
- Dynamic payment location;
- Transaction-related information.

The simplified flow is:

```text
Sale data
   ↓
DynamicQRParams
   ↓
BR Code payload
   ↓
QR Code
```

At the current stage, the project does not include a real banking or PSP infrastructure. Because of that, the generated QR Code represents a simulated PIX payment and cannot be processed by a real financial institution.

## Database

The project uses **SQLite**.

SQLite was chosen because the current project simulates a local card machine. At this stage, adding the complexity of a separate database server would not provide much benefit.

SQLite is lightweight, easy to distribute with the application, and well suited for local applications, embedded software, mobile applications, and prototypes.

### Card Machine

The card machine table stores information used when generating payments, including merchant and owner data.

A Card Machine must be configured before creating a sale.

### Sales

Sales are persisted with information such as:

```text
ID
Sale time
Approval time
Amount
Status
```

Monetary values are stored using integer units instead of floating-point values.

For example:

```text
R$ 25.90
   ↓
2590
```

The decimal representation required by BR Code is rebuilt when needed.

## Sale Status

The main status currently used is:

```text
PENDING
```

The transaction lifecycle will later evolve into:

```text
PENDING
   ├── APPROVED
   ├── REPROVED
   └── EXPIRED
```

These states are part of the next development stages.

## Running the Project

Clone the repository:

```bash
git clone <REPOSITORY_URL>
```

Enter the project directory:

```bash
cd card_machine
```

Install the dependencies:

```bash
go mod tidy
```

Run the application:

```bash
go run .
```

By default, the API runs on:

```text
http://localhost:8080
```

## Important: Card Machine Initialization

Before creating a sale, the Card Machine must be configured.

The sale flow depends on merchant information to build the BR Code payload.

The testing flow should therefore begin with the card machine configuration.

### 1. Configure Card Machine

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

After this step, the application has the required merchant information to generate payments.

## Creating a Sale

### 2. Create Sale

```http
POST /Sale
```

Request body:

```json
{
  "amount": "25.90"
}
```

The amount is received as a `string` so the application can validate the input without depending on floating-point representation.

Internally, the amount is converted to an integer representation before being persisted.

Example:

```text
"25.90"
   ↓
2590
```

When the BR Code payload needs to be generated:

```text
2590
   ↓
"25.90"
```

If the amount cannot be interpreted as a valid monetary value, the API returns `400 Bad Request`.

Invalid example:

```json
{
  "amount": "1ssss"
}
```

Example response:

```json
{
  "error": "invalid amount",
  "result": ""
}
```

## QR Code Generation

After the Sale is created:

1. The sale is persisted;
2. SQLite generates an ID;
3. The ID is used in the dynamic payment location;
4. The BR Code payload is generated;
5. The application generates a PNG QR Code.

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

## Tests

Run all tests with:

```bash
go test -v ./...
```

Run a specific test with:

```bash
go test -v -run TestHttPostSale
```

The current tests cover areas such as database connection, data insertion, and the HTTP sale creation flow.

## Next Steps

The next major step is implementing the transaction lifecycle using Go concurrency features.

After generating the QR Code, the application should wait for one of the possible outcomes:

```text
             Sale PENDING
                  ↓
           Wait for result
             ↙         ↘
      Payment          Timeout
         ↓                ↓
     APPROVED           EXPIRED
```

The implementation will explore concepts such as:

- Goroutines;
- Channels;
- `select`;
- Timers;
- Transaction lifecycle control.

The `REPROVED` status will also be added.

## Future Plans

In later stages, the project will move beyond manual Postman calls.

The goal is to create multiple services communicating with each other and simulate components found in a payment infrastructure.

Conceptual example:

```text
Card Machine
     ↓
Payment Service
     ↓
PSP
     ↓
Simulated PSP / External Service
     ↓
Payment Confirmation
     ↓
Card Machine
```

This evolution will make it possible to explore:

- Service-to-service communication;
- HTTP APIs;
- JSON serialization;
- Asynchronous processing;
- Timeouts;
- Retries;
- Idempotency;
- Concurrency;
- Distributed state.

PIX is currently the main focus of the project, but other payment methods and card brands may be added in the future.

## Goal

The main purpose of this project is to use a real-world problem as a practical backend development laboratory with Go.

More than reproducing a card machine interface, the goal is to understand the flows, protocols, state transitions, and architecture decisions involved in processing a payment transaction.

The project will continue to evolve as new flows are implemented.