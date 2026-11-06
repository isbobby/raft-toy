# Context
This project aims to create a Raft server implemented in Go. Raft is a distributed consensus algorithm which allows stateful servers over an unreliable network to agree on a consistent state.

Three RPCs are required based on the design proposed in the Raft paper, they are
1. `RequestVote`
2. `AppendEntries`
3. `InstallSnapshot`

We will use gRPC over HTTP/2 for node-to-node RPC transport for the following reasons
1. **Contract Safety** and **Minimal transport layer boilerplate**, a single `.proto` contract file generates client, server, and marshalling code — eliminating signature drift (contract safety) and hand-written transport code (boilerplate).
2. **Better performance** as Protobuf's schema-compiled binary encoding avoids the text lexing, key lookup, and reflection costs of JSON decoding, and HTTP/2 multiplexing reduces connection overhead under the high call rate of heartbeats.

## Implementation
### Utilities
We expect frequent updates to the gRPC contract as we continuously add more features. Hence, a `Taskfile` script is created for developers to regenerate the new contract.

The script requires `protoc` and the following `go` dependencies, note that `go env GOPATH` has to be appended to `PATH`:
```shell
export PATH="$PATH:$(go env GOPATH)/bin"

go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.5
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
```

All three RPC methods and payload definitions are defined in the same file so there's a single source of truth.

### Basic Setup <TODO>
1. wiring

2. using of reflection package for debugging

3. verifying wiring is complete

```grpcurl
grpcurl -plaintext -import-path . -proto ./contracts/raft_service.proto -d '{}' localhost:50051 raft.consensus.v1.RaftService/RequestVote

{}
```

## Appendix
### `protobuf` over JSON
JSON requires parsing the entire payload into a token stream and performing a key lookup, whereas protobuf can decode data into the target field without text lexing and key lookup. This is because protobuf replaces field names with a compact integer tag, using a tag-length-value format. Furthermore, when protobuf is used together with `protoc`, the generated code can encode/decode without reflection.

For example, given the following data: `{"user":"alice"}`, a UTF-8 encoded JSON payload has 16 bytes:
```
Wire Bytes (Hex): 7b 22 75 73 65 72 22 3a 22 61 6c 69 63 65 22 7d
ASCII String:     {  "  u  s  e  r  "  :  "  a  l  i  c  e  "  }
```

Whereas protobuf uses binary encoding, the payload is only 7 bytes
```
Wire Bytes (Hex):  0a  05  61 6c 69 63 65
Decoded Meanings: [Header] [Length] [   Value   ]
                   Tag=1      5      " a l i c e "
```

When it comes to unmarshalling into a typed data structure, the raw JSON bytes need to be parsed into tokens, and the "user" field name needs to be matched against the destination attribute name. When it's matched, the value "alice" is decoded from text and then assigned to the destination field.

Unmarshalling Protobuf with `protoc` generated code reads the tag, invokes the corresponding generated typed setter, and decodes the value into the field - without string matching, lexing and reflection.

This yields two performance gains:
1. **smaller payloads**: field names (typically 5-20 bytes, but can be larger) are replaced by tags that are generally smaller (typically under 5 bytes).
2. **fewer decoding cycles**: integer-tag dispatch to generated typed setters avoids lexing, key hashing, and intermediate key→value maps, and (via protoc) reflection.
