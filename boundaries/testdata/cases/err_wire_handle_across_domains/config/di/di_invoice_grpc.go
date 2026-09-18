package di

import (
	invoiceGrpc "example.test/domain/invoice/cmd/grpc"

	"gitlab.com/gorib/di"
)

// The handle comes from the file next door, and this stays inside invoice.
func InitInvoiceGrpc() {
	di.Wire[invoiceGrpc.InvoiceManagement](invoiceManagementInstance)
}
