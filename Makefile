.PHONY: up down connectors templates seed test lint fmt run demo

up:          ## Levanta toda la infraestructura
	docker compose up -d --build

down:        ## Baja todo y borra volúmenes
	docker compose down -v

templates:   ## Aplica las plantillas de índice (ANTES de los conectores)
	curl -sS -X PUT localhost:9200/_index_template/medicines \
		-H 'Content-Type: application/json' -d @connect/index-templates.json
	curl -sS -X PUT localhost:9200/_index_template/availability \
		-H 'Content-Type: application/json' -d @connect/index-template-availability.json

connectors:  ## Registra los conectores de origen y destino
	curl -sS -X POST localhost:8083/connectors \
		-H 'Content-Type: application/json' -d @connect/source-postgres-debezium.json
	curl -sS -X POST localhost:8083/connectors \
		-H 'Content-Type: application/json' -d @connect/sink-opensearch.json

status:      ## Estado de los conectores
	curl -sS localhost:8083/connectors?expand=status | head -c 2000

demo:        ## Flujo completo: infraestructura, plantillas, conectores
	$(MAKE) up && sleep 45 && $(MAKE) templates && sleep 5 && $(MAKE) connectors

test:
	go test ./... -race -count=1

cover:
	go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

lint:
	go vet ./...

fmt:
	gofmt -s -w .

run:
	go run ./cmd/api
