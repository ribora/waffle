//go:generate diogen -output config/diogen/di_gen.go $GOFILE

package di

import (
	invoiceService "example.test/domain/invoice/service"
	rentService "example.test/domain/rent/service"

	"github.com/ribora/diogen/di"
)

func InitDi() {
	di.Wire[rentService.InvoiceCancelling](invoiceService.NewInvoices)
}
