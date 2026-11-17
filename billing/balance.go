package billing

// Balance is the account balance response.
type Balance struct {
	// IsAvailable reports whether the account has funds for API calls.
	IsAvailable  bool          `json:"is_available"`
	BalanceInfos []BalanceInfo `json:"balance_infos"`
}

// Currency is the balance currency.
type Currency string

// Documented currencies.
const (
	CurrencyCNY Currency = "CNY"
	CurrencyUSD Currency = "USD"
)

// BalanceInfo is one currency's balance. Amounts are decimal strings to avoid
// floating-point rounding.
type BalanceInfo struct {
	Currency        Currency `json:"currency,omitempty"`
	TotalBalance    string   `json:"total_balance,omitempty"`
	GrantedBalance  string   `json:"granted_balance,omitempty"`
	ToppedUpBalance string   `json:"topped_up_balance,omitempty"`
}
