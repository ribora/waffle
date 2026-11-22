//go:generate diogen -output config/diogen/di_gen.go $GOFILE

package di

import (
	invoiceService "example.test/domain/invoice/service"

	"github.com/ribora/diogen/di"
)

var invoiceManagementInstance any

func InitInvoiceDomain() {
	invoiceManagementInstance = di.Define(invoiceService.NewInvoiceManagement)
}
