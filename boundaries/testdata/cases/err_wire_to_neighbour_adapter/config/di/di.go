//go:generate diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
	// Unaliased, and the directory is pg while the package is postgres: the
	// qualifier below is the declared name, not the last path segment.
	"example.test/domain/invoice/adapter/pg"
	rentAdapter "example.test/domain/rent/adapter"

	"github.com/ribora/diogen/di"
)

func InitDi() {
	di.Define(postgres.NewInvoiceRepository,
		di.Alias[rentAdapter.Invoices](),
	)
}
