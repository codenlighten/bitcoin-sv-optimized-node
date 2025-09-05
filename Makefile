PROTO_DIR=proto
GEN_DIR=gen

.PHONY: proto
proto:
	buf generate $(PROTO_DIR)
