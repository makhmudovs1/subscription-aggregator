FROM golang:1.23-alpine as builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o server ./cmd/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/server .
COPY .env .
EXPOSE 8080
CMD ["./server"]
