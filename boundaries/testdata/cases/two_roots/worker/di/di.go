//go:generate diogen -output=worker/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
	rentService "example.test/domain/rent/service"

	"gitlab.com/gorib/di"
)

// A second binary's root, and it does not sit under config/.
func InitWorker() {
	di.Define(rentService.NewContracts)
}
