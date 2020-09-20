package controller

import (
	"fmt"

	"gitlab.com/anderstorpsfestivalen/benis-phone/services/backend"
)

type Balance struct {
}

func (m *Balance) Run(c *Controller, k string, menu MenuReturn) MenuReturn {
	fmt.Println(k)
	balance, err := backend.GetBalanceForPhoneNumber(k)
	if err != nil {
		return MenuReturn{
			NextFunction: menu.Caller,
		}
	}

	fmt.Println(balance, err)
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
