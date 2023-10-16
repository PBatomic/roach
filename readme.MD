# Roach
Remote excution service

## Running Roach
Symlink html files to /tmp/static `ln -s path/to/roach/dashboard /tmp/static`
then `go run ./...` or go build cmd/main/main.go and execute binary

By default, service listens on :8081 port. Available endpoints can be found in internal/roachapi.go

## TODO
 - Simple docs & examples
 - Dockerfile and deployment files
 - Authz for requests (secret tokens should be enough for now)
