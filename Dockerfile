FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY . ./


RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -v -o server


FROM alpine:3
RUN apk add --no-cache ca-certificates curl

COPY --from=builder /app/server /server

COPY static /static

EXPOSE 8080

CMD ["./server"]