module github.com/opencord/voltha-openolt-adapter

go 1.12

require (
	github.com/EagleChen/mapmutex v0.0.0-20180418073615-e1a5ae258d8d
	github.com/cenkalti/backoff/v3 v3.1.1
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.1-0.20190118093823-f849b5445de4
	github.com/opencord/voltha-lib-go/v3 v3.0.10
	github.com/opencord/voltha-lib-go/v3 v3.0.9
	github.com/opencord/voltha-protos/v3 v3.2.3
	github.com/opentracing/opentracing-go v1.1.0
	github.com/uber/jaeger-client-go v2.22.1+incompatible
	github.com/uber/jaeger-lib v2.2.0+incompatible // indirect
	go.etcd.io/etcd v0.0.0-20190930204107-236ac2a90522
	google.golang.org/grpc v1.25.1
)
