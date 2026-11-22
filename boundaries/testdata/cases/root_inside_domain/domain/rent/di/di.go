//go:generate diogen -output domain/rent/digen/di_gen.go -pkg digen $GOFILE

package di

import (
	contractRepository "example.test/domain/rent/adapter/postgres"
	rentService "example.test/domain/rent/service"

	"github.com/ribora/diogen/di"
)

// The domain registers itself, so the root sits inside it and names both sides
// the way config/di would.
func InitRentDomain() {
	di.Wire[rentService.ContractRepository](contractRepository.NewContractRepository)
	di.Define(rentService.NewContracts)
}
