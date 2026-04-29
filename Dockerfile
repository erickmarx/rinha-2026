FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

# Copia o restante do código fonte e compila um binário estático.
# CGO_ENABLED=0 desabilita o CGo, garantindo que o binário não dependa de libs do sistema.
# -ldflags="-w -s" remove informações de debug (símbolos e tabela DWARF), reduzindo o tamanho do binário.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o api .

# ESTÁGIO 2: Runtime
# scratch é a imagem mais vazia possível. Não tem shell, não tem SO, nada.
# O container roda APENAS o seu binário. Resultado: imagem mínima e superfície de ataque zero.
FROM scratch

# Copia o binário e os arquivos de dados necessários em tempo de execução.
COPY --from=builder /app/api /api
COPY dataset.bin /dataset.bin
COPY mcc_risk.json /mcc_risk.json
COPY references.json /references.json

EXPOSE 8080

# ENTRYPOINT define o comando padrão ao iniciar o container.
ENTRYPOINT ["/api"]
