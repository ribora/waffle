package convert

import "example.test/domain/rent/model"

func ContractId(contract *model.Contract) int { return contract.Id }
