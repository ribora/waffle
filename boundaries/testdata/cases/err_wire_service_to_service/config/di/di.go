//go:generate diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
	invoiceService "example.test/domain/invoice/service"
	rentService "example.test/domain/rent/service"

	"gitlab.com/gorib/di"
)

func InitDi() {
	di.Wire[rentService.InvoiceCancelling](invoiceService.NewInvoices)
}
