module github.com/synapseai/platform/services/user

go 1.24.0

require (
	github.com/lib/pq v1.10.9
	github.com/synapseai/platform v0.0.0
	go.uber.org/zap v1.26.0
	google.golang.org/grpc v1.64.0
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260209200024-4cfbd4190f57 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/synapseai/platform => ../..
