FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o habanero-agent ./cmd/habanero-agent

FROM alpine:3.19
RUN apk add --no-cache ca-certificates iputils bind-tools tcpdump net-snmp-tools
COPY --from=builder /app/habanero-agent /usr/local/bin/
COPY configs/agent.example.yml /etc/habanero/agent.yml
ENTRYPOINT ["habanero-agent"]
CMD ["--config", "/etc/habanero/agent.yml"]
