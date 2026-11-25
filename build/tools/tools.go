//go:build tools

// Package tools keeps, in go.mod, the tools `make gen` builds or runs at the
// versions pinned by go.mod (see build/gen).
package tools

import (
	_ "github.com/golang/mock/mockgen"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/prysmaticlabs/fastssz/sszgen"
	_ "github.com/prysmaticlabs/protoc-gen-go-cast"
	_ "golang.org/x/tools/cmd/goimports"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
