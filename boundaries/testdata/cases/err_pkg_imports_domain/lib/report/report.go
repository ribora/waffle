package report

import "example.test/domain/invoice/model"

// Not pkg/, and nothing scaffolds this directory — the rule is about being
// outside every domain, not about the name.
func Line(invoice *model.Invoice) string { return invoice.Name }
