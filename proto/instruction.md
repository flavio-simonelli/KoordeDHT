# Compilare i file Protocol Buffers

Questo progetto utilizza **Protocol Buffers** (`.proto`) per definire i servizi gRPC e le strutture dei messaggi.

## Prerequisiti

Assicurati di avere installato:

- Il compilatore `protoc`
- I plugin Go per Protocol Buffers:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

- Aggiungi `$GOPATH/bin` al tuo `PATH` per poter eseguire i plugin:
  ```bash
  export PATH="$PATH:$(go env GOPATH)/bin"
  ```
  
## Compilazione dei file `.proto`
Per compilare i file `.proto`, esegui il seguente comando dalla radice del progetto:

```bashbash
protoc \
-I=proto \
-I=/usr/include \
--go_out=. --go_opt=module=github.com/flaviosimonelli/KoordeDHT \
--go-grpc_out=. --go-grpc_opt=module=github.com/flaviosimonelli/KoordeDHT \
proto/dht/v1/node.proto
```

```bashbash
protoc \
-I=proto \
-I=/usr/include \
--go_out=. --go_opt=module=github.com/flaviosimonelli/KoordeDHT \
--go-grpc_out=. --go-grpc_opt=module=github.com/flaviosimonelli/KoordeDHT \
proto/client/v1/client.proto
```

Questo comando genera i file Go necessari per utilizzare i servizi gRPC definiti nei file `.proto`.
