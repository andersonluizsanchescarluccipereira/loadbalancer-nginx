FROM docker.io/library/golang:1.26-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /lab .
FROM docker.io/library/alpine:3.22
RUN addgroup -S lab && adduser -S -G lab lab
COPY --from=builder /lab /usr/local/bin/lab
USER lab
EXPOSE 7070 8081 8082 8083 9000 9001/udp
ENTRYPOINT ["/usr/local/bin/lab"]
