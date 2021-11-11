.PHONY: build
build:
	go build cmd/k8s-cluster-update-controller.go 
.PHONY: test
test:
	go test ./... -v
