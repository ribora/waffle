//go:generate diogen -pkg diogen $GOFILE

package di

import (
	rentService "example.test/domain/rent/service"

	"gitlab.com/gorib/di"
)

// No -output, so diogen falls back to di_gen.go at the module root.
func InitDi() {
	di.Define(rentService.NewContracts)
}
