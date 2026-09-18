//go:generate diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
	rentService "example.test/domain/rent/service"

	"gitlab.com/gorib/di"
)

func InitDi() {
	di.Define(rentService.NewContracts)
}
