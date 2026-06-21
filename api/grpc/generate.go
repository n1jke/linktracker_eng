package grpc

//go:generate protoc --proto_path=. --go_out=../../internal/infrastructure/transport/grpc/ --go_opt=paths=source_relative --go-grpc_out=../../internal/infrastructure/transport/grpc --go-grpc_opt=paths=source_relative link_tracker.proto
