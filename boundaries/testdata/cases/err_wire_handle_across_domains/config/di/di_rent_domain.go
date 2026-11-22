package di

import (
	rentService "example.test/domain/rent/service"

	"github.com/ribora/diogen/di"
)

// The same handle, answered to a neighbour's service: rent now depends on
// invoice with no import anywhere and no adapter to translate at.
func InitRentDomain() {
	di.Wire[rentService.InvoiceCancelling](invoiceManagementInstance)
}
