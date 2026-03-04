.PHONY: build run test clean deps migrate dev all

APP_NAME := go-ai-agent
BUILD_DIR := bin
CONFIG := etc/config.yaml

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server

all: build-frontend build

run: build
	./$(BUILD_DIR)/$(APP_NAME) -config $(CONFIG)

dev: build-frontend
	go run ./cmd/server -config $(CONFIG)

test:
	go test -v -race ./...

deps:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR) web/dist

migrate:
	@echo "Run: mysql -u root -p go_ai_agent < migrations/001_init.sql"

lint:
	golangci-lint run ./...

dev-frontend:
	cd web && npm run dev

build-frontend:
	cd web && npm run build
