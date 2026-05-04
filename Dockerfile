FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$(go env GOARCH) go build -trimpath -ldflags="-s -w" -o /out/stock ./cmd/server

FROM alpine:3.20

RUN adduser -D -H appuser

WORKDIR /app
COPY --from=builder /out/stock /app/stock

USER appuser
EXPOSE 8080

ENTRYPOINT ["/app/stock"]

