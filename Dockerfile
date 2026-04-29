FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

# Copia APENAS os arquivos fonte necessarios para compilar a API.
# Evita copiar lixo (dataset.bin, .git, docker-compose, lb/, etc.)
# que aumenta o contexto de build e a imagem intermediaria.
COPY main.go .
COPY src/ ./src/

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api .

FROM scratch

COPY --from=builder /app/api /api
COPY dataset.bin /dataset.bin
COPY mcc_risk.json /mcc_risk.json
COPY references.json /references.json

EXPOSE 8080

ENTRYPOINT ["/api"]
