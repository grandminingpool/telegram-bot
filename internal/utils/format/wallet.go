package formatUtils

import (
	"fmt"
	"math/big"
)

var (
	HashrateUnits          = []string{"kH/s", "MH/s", "GH/s", "TH/s", "PH/s", "EH/s"}
	HashrateUnitStep int64 = 1000
)

func WalletBalance(balance uint64, atomicUnit uint16) string {
	balanceFormatted := float64(balance) / float64(atomicUnit)

	return fmt.Sprintf("%.2f", balanceFormatted)
}

func Hashrate(hashrate *big.Int) string {
	c := big.NewInt(1)
	i := 0

	for idx := range HashrateUnits {
		c.Mul(c, big.NewInt(HashrateUnitStep))

		if hashrate.Cmp(c) == -1 {
			i = idx
		}
	}

	h := *hashrate
	h.Div(hashrate, c)
	hf, _ := h.Float64()

	return fmt.Sprintf("%.2f %s", hf, HashrateUnits[i])
}
