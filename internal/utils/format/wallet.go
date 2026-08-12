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
		"zcash":        "Sol/s",
		"mimblewimble": "Gps",
	}
	hashrateStep = big.NewFloat(1000)
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

	h := new(big.Float).SetInt(hashrate)
	prefix := ""

	for _, p := range hashratePrefixes {
		if h.Cmp(hashrateStep) < 0 {
			break
		}
		h.Quo(h, hashrateStep)
		prefix = p
	}

	hf, _ := h.Float64()
	return fmt.Sprintf("%.2f %s%s", hf, prefix, unit)
}
