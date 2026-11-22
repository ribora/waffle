package di

import (
	invoiceGrpc "example.test/domain/invoice/cmd/grpc"

	"github.com/ribora/diogen/di"
)

// The handle comes from the file next door, and this stays inside invoice.
func InitInvoiceGrpc() {
	di.Wire[invoiceGrpc.InvoiceManagement](invoiceManagementInstance)
}
