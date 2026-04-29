# ESTÁGIO 1: Builder
# Mesma estratégia da API: imagem completa do Go para compilar.
FROM golang:1.23-alpine AS builder
WORKDIR /app

# Copia apenas o código fonte do load balancer (único arquivo necessário).
COPY main.go .

# Compila binário estático e enxuto.
# Não precisamos de go.mod porque o lb/main.go usa apenas a biblioteca padrão.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o lb .

# ESTÁGIO 2: Runtime
# scratch novamente: a imagem final tem apenas o binário do LB.
FROM scratch

COPY --from=builder /app/lb /lb

EXPOSE 8080

ENTRYPOINT ["/lb"]
