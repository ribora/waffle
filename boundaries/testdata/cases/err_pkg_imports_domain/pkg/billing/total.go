package billing

import "example.test/domain/invoice/model"

func Total(invoices []*model.Invoice) int {
	var sum int
	for _, invoice := range invoices {
		sum += invoice.Amount
	}
	return sum
}
