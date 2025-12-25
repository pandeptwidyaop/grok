#!/bin/bash
set -e

echo "Generating gRPC code from proto files..."

# Create output directory
mkdir -p gen/go/tunnel/v1

# Generate Go code from proto files
protoc --go_out=gen --go_opt=paths=source_relative \
    --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
    proto/tunnel/v1/*.proto

echo "gRPC code generation complete!"
