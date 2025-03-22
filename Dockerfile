FROM golang:1.19-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download || echo "Downloading dependencies during build"
COPY src/ ./src/
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o dipa-auto ./src

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
RUN mkdir -p /var/lib/dipa-auto
COPY --from=builder /app/dipa-auto /app/dipa-auto
COPY setup.sh /app/setup.sh
RUN chmod +x /app/dipa-auto /app/setup.sh
ENTRYPOINT ["/app/setup.sh"]
