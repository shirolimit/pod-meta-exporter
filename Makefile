build:
	go build -o pod-meta-exporter ./cmd/pod-meta-exporter

build_static:
	go build -ldflags "-linkmode external -extldflags -static" -o pod-meta-exporter ./cmd/pod-meta-exporter

test:
	go test ./... -v -race -cover -coverprofile=coverage.txt && go tool cover -func=coverage.txt

build_docker:
	docker build -f docker/Dockerfile -t shirolimit/pod-meta-exporter .
