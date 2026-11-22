//go:generate diogen -output=worker/diogen/di_gen.go $GOFILE

package di

import (
	rentService "example.test/domain/rent/service"

	"github.com/ribora/diogen/di"
)

// A second binary's root, and it does not sit under config/.
func InitWorker() {
	di.Define(rentService.NewContracts)
}
