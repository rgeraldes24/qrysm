//go:build tools

package main

// The generators build/gen builds from go.mod's requirements. Importing them
// here keeps `go mod tidy` from dropping the modules and go.sum entries they
// need.
import (
	_ "github.com/golang/mock/mockgen"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/prysmaticlabs/fastssz/sszgen"
	_ "github.com/prysmaticlabs/protoc-gen-go-cast"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
