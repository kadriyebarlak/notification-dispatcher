run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

test-race:
	go test -race ./...

DB_URL=postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DB_URL)" status

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-build:
	docker build -t notification-dispatcher .

docker-run:
	docker run --rm \
		-e DATABASE_URL=postgres://notify:notify@host.docker.internal:5432/notification_dispatcher?sslmode=disable \
		-p 8080:8080 \
		notification-dispatcher