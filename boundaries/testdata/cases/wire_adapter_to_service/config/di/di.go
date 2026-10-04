//go:generate diogen -output config/diogen/di_gen.go $GOFILE

package di

import (
	invoiceService "example.test/domain/invoice/service"
	"example.test/domain/rent/adapter"

	"gitlab.com/gorib/di"
)

func InitDi() {
	di.Define(invoiceService.NewInvoices,
		di.Alias[adapter.InvoiceManagement](),
	)
}
