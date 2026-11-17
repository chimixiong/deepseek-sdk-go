// Command balance prints the account balance.
//
//	go run ./examples/balance
package main

import (
	"context"
	"fmt"

	ex "github.com/chimixiong/deepseek-sdk-go/examples/internal/exutil"

	"github.com/chimixiong/deepseek-sdk-go/billing"
)

func main() {
	balance, err := billing.New(ex.NewClient()).GetBalance(context.Background())
	ex.Must(err)

	fmt.Printf("available: %v\n", balance.IsAvailable)
	for _, info := range balance.BalanceInfos {
		fmt.Printf("%s total=%s granted=%s topped_up=%s\n",
			info.Currency, info.TotalBalance, info.GrantedBalance, info.ToppedUpBalance)
	}
}
