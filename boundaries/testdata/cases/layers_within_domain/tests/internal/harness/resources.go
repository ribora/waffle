package harness

import (
	"example.test/domain/rent/adapter/rf"
	"example.test/domain/rent/cmd/grpc"
)

// The harness reaches past every layer on purpose: it builds the schema and
// boots the real container.
type App struct {
	Policy   *rf.Policy
	Contract grpc.ContractService
}
