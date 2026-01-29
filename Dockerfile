FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache \
    gcc \
    musl-dev \
    sqlite-dev

COPY . .

RUN go mod tidy

RUN CGO_ENABLED=1 GOOS=linux go build -o /app/main ./main.go

FROM alpine:latest

RUN apk add --no-cache \
    ca-certificates \
    bash \
    sqlite-libs \
    curl

COPY --from=builder /app/main /app/main

CMD ["/app/main"]
