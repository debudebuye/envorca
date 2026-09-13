module runorka.dev/runorka/cli

go 1.27

require (
	github.com/spf13/cobra v1.10.2
	google.golang.org/grpc v1.83.2
	runorka.dev/runorka/api v0.0.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

replace runorka.dev/runorka/api => ../api
