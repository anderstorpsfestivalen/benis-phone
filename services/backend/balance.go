package backend

type BalanceResp {
	Balance float64 `json:"balance"`
}

func GetBalanceForPhoneNumber(number string) {
	req, err := http.NewRequest("POST", "http://localhost/", nil)
}