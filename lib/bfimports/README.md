# README

- Author: [LMJW](https://github.com/LMJW)

---

This `imports` directory is mainly used for saving all the golang sturctures and protobuf generated files.

--- 
- How to compile and generate the protobuf files
    1. go to the `/go-bftx/lib` directory
    2. Open terminal, run command
        `protoc -Iproto.source/Bftransaction -Iproto.source/saberproto --go_out=plugins=grpc:bfimports/ Bftransaction.proto saberservice.proto`