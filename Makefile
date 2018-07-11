.PHONY: storage_rpc clean_storage_rpc clean_rpc rpc metadata_rpc clean_metadata_rpc

clean_rpc: clean_storage_rpc clean_metadata_rpc

rpc: storage_rpc metadata_rpc

storage_rpc:
	protoc -I=pkg/rpc/storage --go_out=plugins=grpc:pkg/rpc/storage pkg/rpc/storage/storage.proto

clean_storage_rpc:
	rm pkg/rpc/storage/storage.pb.go

metadata_rpc:
	protoc -I=pkg/rpc/metadata --go_out=plugins=grpc:pkg/rpc/metadata pkg/rpc/metadata/metadata.proto

clean_metadata_rpc:
	rm pkg/rpc/metadata/metadata.pb.go