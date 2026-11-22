//go:generate diogen -output config/diogen/di_gen.go $GOFILE

package di

import (
	invoiceService "example.test/domain/invoice/service"
	"example.test/domain/rent/adapter"

	"github.com/ribora/diogen/di"
)

func InitDi() {
	di.Define(invoiceService.NewInvoices,
		di.Alias[adapter.InvoiceManagement](),
	)
}
