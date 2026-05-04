# Simplified Stock Market Service

Highly available stock market simulator with:
- **Wallets** that hold stocks
- **Bank** as the sole liquidity provider
- **Audit log** of successful wallet buy/sell operations only

The service runs multiple API instances behind a load balancer and SSOT database. 
`POST /chaos` kills only the instance that serves that request; the service remains available through other instances.

## Tech stack

- Go (HTTP API)
- PostgreSQL (shared durable state)
- Nginx (load balancer)
- Docker Compose (orchestration)

## Code structure

- `cmd/server/main.go` - process wiring and startup
- `internal/httpapi` - HTTP routing, request/response mapping
- `internal/service` - business rules and validations
- `internal/store/postgres` - database access and transactions
- `internal/model` - API/domain DTOs
- `internal/domain` - shared operation constants and domain errors

## Start

### Linux/macOS

```bash
./start.sh 8080
```

### Windows (PowerShell)

```powershell
.\start.ps1 8080
```

Service URL:

```text
http://localhost:8080
```

Stop:

```bash
./stop.sh
```

or

```powershell
.\stop.ps1
```

## Running tests
```bash
go test ./...
```

## Nix development shell (optional)

If your environment is missing tools and you have Nix package manager, use:

```bash
nix develop
```

## Running tests

```bash
nix develop -c go test ./...
```

## API

### 1. Wallet operation

`POST /wallets/{wallet_id}/stocks/{stock_name}`  
Body:

```json
{"type":"buy"}
```

or

```json
{"type":"sell"}
```

Rules:
- If wallet does not exist, it is created
- If stock does not exist, returns `404`
- `buy` with no stock in bank returns `400`
- `sell` with no stock in wallet returns `400`
- Success returns `200`
- Successful operations are appended to audit log

### 2. Get wallet state

`GET /wallets/{wallet_id}`

Response:

```json
{"id":"wallet-1","stocks":[{"name":"stock1","quantity":2}]}
```

Returns `404` when wallet does not exist.

### 3. Get wallet stock quantity

`GET /wallets/{wallet_id}/stocks/{stock_name}`

Response body is a single number, e.g.:

```text
99
```

Returns:
- `404` when wallet does not exist
- `404` when stock does not exist
- `200` with `0` when wallet exists but does not own that stock

### 4. Get bank state

`GET /stocks`

Response:

```json
{"stocks":[{"name":"stock1","quantity":99},{"name":"stock2","quantity":1}]}
```

### 5. Set bank state

`POST /stocks`

Body:

```json
{"stocks":[{"name":"stock1","quantity":99},{"name":"stock2","quantity":1}]}
```

Success returns `200`.

Behavior:
- Stocks in the payload are created/updated with provided quantities
- Existing known stocks not present in payload are set to quantity `0`

### 6. Get audit log

`GET /log`

Response:

```json
{"log":[{"type":"buy","wallet_id":"wallet-1","stock_name":"stock1"}]}
```

Only successful wallet operations are included. Bank operations are excluded.

### 7. Chaos endpoint

`POST /chaos`

Returns `200` and terminates the serving API instance shortly after response.
