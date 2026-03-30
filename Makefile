.PHONY: build test install docker clean

VERSION ?= 0.1.0
BINARY = habanero-agent

build:
	CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) ./cmd/habanero-agent

test:
	go test ./...

install: build
	sudo cp $(BINARY) /usr/local/bin/
	sudo mkdir -p /etc/habanero
	sudo cp configs/agent.example.yml /etc/habanero/agent.yml
	sudo cp systemd/habanero-agent.service /etc/systemd/system/
	sudo systemctl daemon-reload

docker:
	docker build -t cosmicllama/habanero-agent:$(VERSION) .

clean:
	rm -f $(BINARY)
