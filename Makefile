SWAG_VERSION ?= v1.16.4
SWAG := go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
GOCACHE ?= $(CURDIR)/.cache/go-build

.PHONY: swagger test build docker-up

swagger:
	$(SWAG) init -g cmd/api/main.go -o docs --parseInternal --parseDependency

test:
	GOCACHE=$(GOCACHE) go test ./...

build:
	GOCACHE=$(GOCACHE) go build ./cmd/api

docker-up:
	docker compose up --build -d
