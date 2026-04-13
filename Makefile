.PHONY: help test test-go test-worker test-js fmt run dev db-dev db-migrate doctor sample-media

help:
	@printf "Targets:\\n"
	@printf "  make test        Run all checks\\n"
	@printf "  make test-go     Run Go tests\\n"
	@printf "  make test-worker Run Python worker tests\\n"
	@printf "  make test-js     Check frontend JS syntax\\n"
	@printf "  make fmt         Format Go sources\\n"
	@printf "  make run         Run server with current env\\n"
	@printf "  make dev         Run server in local no-DB mode\\n"
	@printf "  make db-dev      Run server with DB + worker enabled\\n"
	@printf "  make db-migrate  Run server with DB migrations enabled\\n"
	@printf "  make doctor      Check local runtime prerequisites\\n"
	@printf "  make sample-media Generate local sample media files\\n"

test: test-go test-worker test-js

test-go:
	go test ./...

test-worker:
	python3 -m unittest discover -s worker -p '*_test.py'

test-js:
	node --check internal/httpserver/assets/app.js

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -print)

run:
	go run ./cmd/server

dev:
	IDEA_ENABLE_DATABASE=false IDEA_RUN_MIGRATIONS=false go run ./cmd/server

db-dev:
	IDEA_ENABLE_DATABASE=true IDEA_RUN_MIGRATIONS=false IDEA_RUN_JOB_WORKER=true go run ./cmd/server

db-migrate:
	IDEA_ENABLE_DATABASE=true IDEA_RUN_MIGRATIONS=true IDEA_RUN_JOB_WORKER=false go run ./cmd/server

doctor:
	go run ./cmd/devdoctor

sample-media:
	go run ./cmd/devsample
