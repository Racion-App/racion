package planner

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"racion/internal/i18n"
)

// Money — сумма в валюте страны: «1 234 ₽», «12,50 Br», «1 500 ₸», «12,50 €», «$12.50», «£3.20», «45 kr».
func (c Country) Money(v float64) string {
	v = c.RoundMoney(v)
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int64(math.Floor(v + 1e-9))
	frac := int64(math.Round((v - float64(whole)) * math.Pow(10, float64(c.Decimals))))
	if c.Decimals > 0 && frac >= int64(math.Pow(10, float64(c.Decimals))) {
		whole++
		frac = 0
	}
	s := strconv.FormatInt(whole, 10)
	thou, dec := c.ThouSep, c.DecSep
	if thou == "" {
		thou = " "
	}
	if dec == "" {
		dec = ","
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(thou)
		}
		b.WriteRune(ch)
	}
	num := b.String()
	if c.Decimals > 0 {
		num += dec + fmt.Sprintf("%0*d", c.Decimals, frac)
	}
	if neg {
		num = "−" + num
	}
	if c.SymbolBefore {
		return c.Symbol + num
	}
	return num + " " + c.Symbol
}

// FormatQty — количество с единицей на языке: «400 г», «1,5 кг», «2 шт».
func FormatQty(l i18n.Lang, v float64, unit string) string {
	dec := i18n.Meta(l).Decimal
	if dec == "" {
		dec = ","
	}
	switch unit {
	case "pcs":
		if v == math.Trunc(v) {
			return fmt.Sprintf("%d %s", int(v), i18n.T(l, "unit.pcs"))
		}
		return strings.Replace(fmt.Sprintf("%.1f %s", v, i18n.T(l, "unit.pcs")), ".", dec, 1)
	case "ml":
		if v >= 1000 {
			return strings.Replace(fmt.Sprintf("%.2g %s", v/1000, i18n.T(l, "unit.l")), ".", dec, 1)
		}
		if v > 0 && v < 1 {
			return strings.Replace(fmt.Sprintf("%.1f %s", v, i18n.T(l, "unit.ml")), ".", dec, 1)
		}
		return fmt.Sprintf("%d %s", int(math.Round(v)), i18n.T(l, "unit.ml"))
	default:
		if v >= 1000 {
			return strings.Replace(fmt.Sprintf("%.2g %s", v/1000, i18n.T(l, "unit.kg")), ".", dec, 1)
		}
		if v > 0 && v < 1 {
			return strings.Replace(fmt.Sprintf("%.1f %s", v, i18n.T(l, "unit.g")), ".", dec, 1)
		}
		return fmt.Sprintf("%d %s", int(math.Round(v)), i18n.T(l, "unit.g"))
	}
}
