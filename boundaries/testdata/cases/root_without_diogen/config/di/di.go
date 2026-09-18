package di

import (
	invoiceRepository "example.test/domain/invoice/adapter/postgres"
	invoiceService "example.test/domain/invoice/service"
)

// No //go:generate line and no di import: the service opted out of diogen and
// constructs in order by hand. Nothing in here says it is the root, so the test
// that runs the check names it.
func InitDi() *invoiceService.Invoices {
	return invoiceService.NewInvoices(invoiceRepository.NewInvoiceRepository())
}
