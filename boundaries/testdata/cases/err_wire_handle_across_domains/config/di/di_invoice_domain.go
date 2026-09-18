//go:generate diogen -output config/diogen/di_gen.go -pkg diogen $GOFILE

package di

import (
	invoiceService "example.test/domain/invoice/service"

	"gitlab.com/gorib/di"
)

var invoiceManagementInstance any

func InitInvoiceDomain() {
	invoiceManagementInstance = di.Define(invoiceService.NewInvoiceManagement)
}
