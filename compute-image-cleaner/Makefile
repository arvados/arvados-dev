
build:
	@go build -ldflags "-s -w"

lint:
	@gofmt -s -w *go
	@golint
	@cd config/; golint
	@golangci-lint run
	@cd config/; golangci-lint run

