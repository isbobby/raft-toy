# Getting Started: gRPC Contracts

## Context
This project aims to create a Raft server implemented in Go. Raft is a distributed consensus algorithm which allows stateful servers over an unreliable network to agree on a consistent state.

The Raft paper defines three RPCs: two for the core algorithm, and one for log compaction:
1. `RequestVote`
2. `AppendEntries`
3. `InstallSnapshot` (log compaction)

We will use gRPC over HTTP/2 for node-to-node RPC transport for the following reasons:
1. **Contract safety** and **minimal transport-layer boilerplate**: a single `.proto` contract file generates client, server, and marshalling code, eliminating signature drift (contract safety) and hand-written transport code (boilerplate).
2. **Better performance**: Protobuf's schema-compiled binary encoding avoids the text lexing, field-name matching, and reflection costs of JSON decoding, and HTTP/2 multiplexing reduces connection overhead under the high call rate of heartbeats.

## Implementation
### Utilities
We expect frequent updates to the gRPC contract as we add more features. Hence, a `Taskfile` task is provided for developers to regenerate Go code from the contract:
```shell
task proto:gen
```

The task requires `protoc` and the following Go plugins. `$(go env GOPATH)/bin` must be on your `PATH`:
```shell
export PATH="$PATH:$(go env GOPATH)/bin"

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
```

### Basic Setup
#### Defining the contract
All three RPC methods and their message definitions live in `contracts/raft_service.proto`, so there's a single source of truth.

`raft_service.proto` declares a `package` and a `service`. Together with each `rpc` name, they form the request path `/<package>.<Service>/<Method>`. The server uses this path to route each call. For example:

```protobuf
package raft.consensus.v1;

service RaftService {
  rpc RequestVote(RequestVoteRequest) returns (RequestVoteResponse);
}
```

serves `RequestVote` at `/raft.consensus.v1.RaftService/RequestVote`.

#### Wiring the server
`src/main.go` wires the generated code to our implementation in `src/internal/raft`:
1. `net.Listen("tcp", ":50051")` opens the listener.
2. `grpc.NewServer()` creates the gRPC server.
3. `pb.RegisterRaftServiceServer(grpcServer, raft.NewService())` registers our implementation under `raft.consensus.v1.RaftService`.
4. `reflection.Register(grpcServer)` enables server reflection (see below).
5. `grpcServer.Serve(lis)` blocks and serves requests.

All services must be registered before `Serve` is called.

#### gRPC server reflection
`google.golang.org/grpc/reflection` is unrelated to Go's `reflect` package. `reflect` inspects Go types at runtime. gRPC server reflection lets a client inspect a running server's services and message schemas.

`reflection.Register(grpcServer)` registers an extra service, `grpc.reflection.v1.ServerReflection`. Clients call it to list the registered services (`list_services`) and to fetch the schema (file descriptors) for any service or message.

Server reflection is a language-neutral protocol, so any client that speaks it can query a server written in any language. Each language implements the server side separately, and it must be enabled explicitly.

With reflection enabled, `grpcurl` can discover and call endpoints without the `.proto` file:
```shell
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{}' localhost:50051 raft.consensus.v1.RaftService/RequestVote
```

Without reflection, pass the contract explicitly:
```shell
grpcurl -plaintext -import-path . -proto ./contracts/raft_service.proto -d '{}' localhost:50051 raft.consensus.v1.RaftService/RequestVote
```

## Appendix
### `protobuf` over JSON
JSON decoding requires lexing the payload into tokens and matching each field name against the destination struct. Protobuf avoids both: it replaces field names with compact integer tags in a tag-length-value format, so the decoder can map each value to its target field by number.

For example, given the following data: `{"user":"alice"}`, a UTF-8 encoded JSON payload has 16 bytes:
```
Wire Bytes (Hex): 7b 22 75 73 65 72 22 3a 22 61 6c 69 63 65 22 7d
ASCII String:     {  "  u  s  e  r  "  :  "  a  l  i  c  e  "  }
```

With protobuf's binary encoding, the same payload is 7 bytes:
```
Wire Bytes (Hex):  0a                  05        61 6c 69 63 65
Decoded Meanings: [Tag]               [Length]  [   Value    ]
                   field=1, type=LEN   5         " a l i c e "
```

The tag byte packs the field number and wire type together: `0x0a = (1 << 3) | 2`, i.e. field 1, wire type 2 (length-delimited).

When unmarshalling JSON into a typed data structure, the raw bytes are parsed into tokens, and the `"user"` field name is matched against the destination field name. Once matched, the value `"alice"` is decoded from text and assigned to the field.

When unmarshalling protobuf, the decoder reads each tag's field number and writes the value directly into the matching struct field, using a field table built once at startup. It needs no text parsing and no field-name matching.

This yields two performance gains:
1. **Smaller payloads**: field names (typically 5–20 bytes, but can be larger) are replaced by tags of 1 byte for fields 1–15 and 2 bytes for fields 16–2047. Frequently used fields should therefore get numbers 1–15.
2. **Fewer decoding cycles**: dispatching on integer field numbers avoids lexing, field-name hashing, and intermediate key→value maps.

### gRPC and RPC
Birrell & Nelson's seminal paper, "Implementing Remote Procedure Calls" (1984), established:
1. generated stubs that implement the client and server sides of a shared interface
2. no emulated shared memory: arguments and results are passed by copy, never by address
3. binding (locating a server for an interface) via Grapevine, a distributed registration database
4. the goal of making remote calls look as much like local calls as possible

gRPC, which grew out of Google's internal RPC system Stubby, keeps [1] and [2] ("messages, not references"), and revises [3] and [4].

For [3], binding is pluggable: a name resolver (DNS by default; xDS or custom) maps a target to addresses, and a client-side load-balancing policy picks one per call.

For [4], gRPC makes the network explicit. A remote call is slower than a local one, can fail partway, and the client may not know whether a failed call ran on the server. So deadlines, cancellation and status codes are part of the protocol. In Go these appear in every method signature as `context.Context` (deadline and cancellation) and a returned `error` (a gRPC status).

Retries are opt-in, configured per method. Whether a retry is safe is up to the application: Raft's RPCs are designed so that a repeated call is harmless.
