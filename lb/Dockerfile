FROM golang:1.23-alpine AS builder
WORKDIR /app

COPY go.mod .
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o lb .

FROM scratch
COPY --from=builder /app/lb /lb
EXPOSE 8080
ENTRYPOINT ["/lb"]
