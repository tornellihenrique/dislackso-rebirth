module github.com/tornellihenrique/dislackso-rebirth/hub

go 1.27.2

tool (
	connectrpc.com/connect/v2/cmd/protoc-gen-connect-go
	google.golang.org/protobuf/cmd/protoc-gen-go
)

require (
	connectrpc.com/connect/v2 v2.0.0
	github.com/google/go-cmp v0.7.0
	go.yaml.in/yaml/v3 v3.0.5
	google.golang.org/protobuf v1.36.12
)
