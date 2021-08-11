module github.com/opencord/voltha-openolt-adapter

go 1.16

replace github.com/opencord/voltha-protos/v4 => /Users/knursimu/work/go/src/github.com/opencord/voltha-protos

replace github.com/opencord/voltha-lib-go/v6 => /Users/knursimu/work/go/src/github.com/opencord/voltha-lib-go

replace github.com/coreos/bbolt v1.3.4 => go.etcd.io/bbolt v1.3.4

replace go.etcd.io/bbolt v1.3.4 => github.com/coreos/bbolt v1.3.4

replace google.golang.org/grpc => google.golang.org/grpc v1.25.1

require (
	github.com/cenkalti/backoff/v3 v3.1.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/opencord/voltha-lib-go/v6 v6.0.1
	github.com/opencord/voltha-protos/v4 v4.2.0
	go.etcd.io/etcd v3.3.25+incompatible
	google.golang.org/grpc v1.39.1
)
