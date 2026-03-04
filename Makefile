.PHONY: build run test clean deps migrate dev

APP_NAME := go-ai-agent
BUILD_DIR := bin
CONFIG := etc/config.yaml

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

run: build
	./$(BUILD_DIR)/$(APP_NAME) -config $(CONFIG)

dev:
	go run ./cmd/server -config $(CONFIG)

test:
	go test -v -race ./...

deps:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)

migrate:
	@echo "Run: mysql -u root -p go_ai_agent < migrations/001_init.sql"

lint:
	golangci-lint run ./...

dev-frontend:
	cd web && npm run dev

build-frontend:
	cd web && npm run build
