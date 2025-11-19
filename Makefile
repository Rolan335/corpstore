MIGRATIONS_DIR := ./migrations
DSN := postgres://corpuser:corppass@localhost:5432/corpstore?sslmode=disable

migrate-up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DSN)" up

migrate-down:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DSN)" down

migrate-status:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DSN)" status

create-migration:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DSN)" create $(NAME) sql