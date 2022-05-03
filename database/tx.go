package database

type Account string

type Tx struct {
	From  Account `json:"from"`
	To    Account `json:"to"`
	Value uint    `json:"value"`
	Data  string  `json:"data"`
}

func NewTx(from Account, to Account, value uint, data string) Tx {
	return Tx{from, to, value, data}
}

func (t Tx) IsReward() bool {
	return t.Data == "reward"
}
