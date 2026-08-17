// Package money provides integer-JPY money arithmetic shared by every
// pricing code path. The application forbids floating-point money.
package money

// RoundJPY returns the supplied integer JPY amount unchanged. The
// function is the documented single rounding point used by every
// pricing path so interactive and batch decisions agree. A future
// change that needs fractional-JPY rounding must update both call
// sites together.
func RoundJPY(amountJPY int64) int64 {
	return amountJPY
}

// ApplyPercent returns the discount (in JPY) produced by percentBP
// basis points of amount. percentBP=100 means 1%, percentBP=10000
// means 100%. Integer division is used so the result is always
// whole JPY.
func ApplyPercent(amountJPY int64, percentBP int) int64 {
	if amountJPY <= 0 || percentBP <= 0 {
		return 0
	}
	discount := (amountJPY * int64(percentBP)) / 10000
	return RoundJPY(discount)
}

// MaxJPY returns the larger of two JPY amounts.
func MaxJPY(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// MinJPY returns the smaller of two JPY amounts.
func MinJPY(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
