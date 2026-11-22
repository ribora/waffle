//go:generate diogen -output config/diogen/di_gen.go $GOFILE

package di

import (
	rentService "example.test/domain/rent/service"

	"github.com/ribora/diogen/di"
)

func InitDi() {
	di.Define(rentService.NewContracts)
}
