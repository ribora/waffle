package service

import (
	"testing"

	invoiceModel "example.test/domain/invoice/model"
)

func TestBill(t *testing.T) {
	_ = invoiceModel.Invoice{Amount: 100}
}
