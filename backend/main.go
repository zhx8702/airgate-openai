package main

import (
	"github.com/DouDOU-start/airgate-openai/backend/internal/gateway"
	sdkgrpc "github.com/DouDOU-start/airgate-sdk/grpc"
)

func main() {
	sdkgrpc.Serve(&gateway.OpenAIGateway{})
}
