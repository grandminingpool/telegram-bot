package format

import (
	"fmt"
	"math"
	"math/big"
)

const defaultHashrateUnit = "H/s"

var (
	hashratePrefixes = []string{"k", "M", "G", "T", "P", "E"}
	hashrateUnits    = map[string]string{
		"zcash": "Sol/s",
	}
)

func HashrateUnit(coin string) string {
	if unit, ok := hashrateUnits[coin]; ok {
		return unit
	}

	return defaultHashrateUnit
}

func WalletBalance(balance uint64, atomicUnit uint16) string {
	balanceFormatted := float64(balance) / math.Pow(10, float64(atomicUnit))

	return fmt.Sprintf("%.2f", balanceFormatted)
}

func Hashrate(hashrate *big.Int, coin string) string {
	unit := HashrateUnit(coin)

	if hashrate.Sign() == 0 {
		return fmt.Sprintf("0.00 %s", unit)
	}

	step := big.NewFloat(1000)
	h := new(big.Float).SetInt(hashrate)

	for _, prefix := range hashratePrefixes {
		h.Quo(h, step)
		if h.Cmp(step) < 0 {
			hf, _ := h.Float64()
			return fmt.Sprintf("%.2f %s%s", hf, prefix, unit)
		}
	}

	hf, _ := h.Float64()
	return fmt.Sprintf("%.2f %s%s", hf, hashratePrefixes[len(hashratePrefixes)-1], unit)
}
