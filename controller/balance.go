package controller

import (
	"fmt"
)

type Balance struct {
}

func (m *Balance) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	balance.GetBalanceForPhoneNumber(k)
	fmt.Println(k)
	return MenuReturn{
		NextFunction: menu.Caller,
	}

}

func (m *Balance) InputLength() int {
	return 10
}

func (m *Balance) Name() string {
	return "balance"
}
