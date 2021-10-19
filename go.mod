module github.com/opencord/voltha-openolt-adapter

go 1.16

replace (
	github.com/coreos/bbolt v1.3.4 => go.etcd.io/bbolt v1.3.4
	github.com/opencord/voltha-lib-go/v7 => ../voltha-lib-go
	github.com/opencord/voltha-protos/v5 => ../voltha-protos
	go.etcd.io/bbolt v1.3.4 => github.com/coreos/bbolt v1.3.4
	google.golang.org/grpc => google.golang.org/grpc v1.25.1
)

require (
	github.com/cenkalti/backoff/v3 v3.1.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/opencord/voltha-lib-go/v7 v7.0.1
	github.com/opencord/voltha-protos/v5 v5.0.0
	go.etcd.io/etcd v3.3.25+incompatible
	google.golang.org/grpc v1.41.0
)
