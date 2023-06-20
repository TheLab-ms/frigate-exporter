FROM golang:1.19 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build

FROM scratch
COPY --from=builder /app/frigate-exporter /frigate-exporter
ENTRYPOINT ["/frigate-exporter"]