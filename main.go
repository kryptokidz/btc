package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kryptokidz/btc/coinbase"
)

func main() {
	var (
		sinceDate string
		all       bool
		printSpot bool
	)
	flag.StringVar(&sinceDate, "since", "", "ISO-8601 date")
	flag.BoolVar(&all, "all", false, "show earnings since zero time")
	flag.BoolVar(&printSpot, "spot", false, "include currency spot rates")

	flag.Parse()

	if sinceDate != "" && all {
		log.Fatal("one of -since or -all")
	}

	var since time.Time
	if !all {
		since = time.Now().Add(-4 * 7 * 24 * time.Hour) // last 4 weeks
	}
	if sinceDate != "" {
		s, err := time.Parse("2006-01-02", sinceDate)
		if err != nil {
			log.Fatal(err)
		}
		since = s
	}

	holdings, spot := calcHoldings(since)
	if printSpot {
		printSpotRates(spot)
		fmt.Println()
	}
	printHoldings(holdings)
}

func printSpotRates(spot []*coinbase.SpotRate) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	headers := []string{"Spot Rate", "$"}
	printLine(w, headers)
	printSep(w, headers)
	for _, spot := range spot {
		printLine(w, []string{
			spot.Base,
			fmtUSD(spot.Amount()),
		})
	}
	_ = w.Flush()
}

func printHoldings(holdings []*Holding) {
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	headers := []string{"Holding", "Cost Basis", "Amount", "$", "+/-", "%"}
	printLine(w, headers)
	printSep(w, headers)

	var sums struct {
		costBasis, nativeValue float64
	}
	for _, holding := range holdings {
		sums.costBasis += holding.CostBasis
		sums.nativeValue += holding.NativeValue
		printLine(w, []string{
			holding.Currency,
			fmtUSD(holding.CostBasis),
			fmtVal(holding.Value),
			fmtUSD(holding.NativeValue),
			sign(holding.Profit()) + fmtUSD(math.Abs(holding.Profit())),
			sign(holding.ProfitPercent()) + fmtPCT(math.Abs(holding.ProfitPercent())),
		})
	}
	profit := sums.nativeValue - sums.costBasis
	profitPCT := profit / sums.costBasis
	printSep(w, headers)

	printLine(w, []string{
		"Total",
		fmtUSD(sums.costBasis), // cost basis
		"", // value (crypto)
		fmtUSD(sums.nativeValue), // total usd value
		sign(profit) + fmtUSD(math.Abs(profit)),
		sign(profit) + fmtPCT(math.Abs(profitPCT)),
	})

	_ = w.Flush()
}

func sign(val float64) string {
	if math.IsInf(val, 0) {
		return ""
	}
	if val < 0 {
		return "-"
	}
	return "+"
}

func fmtPCT(val float64) string {
	return fmtVal(val*100) + "%"
}

func fmtUSD(val float64) string {
	return "$" + fmtVal(val)
}

func fmtVal(val float64) string {
	result := fmt.Sprintf("%.2f", val)
	for i := strings.LastIndex(result, ".") - 3; i > 0; i -= 3 {
		result = result[0:i] + "," + result[i:len(result)]
	}
	return result
}

type Holding struct {
	Currency    string
	Value       float64
	CostBasis   float64
	NativeValue float64
}

func (g *Holding) Profit() float64 {
	return g.NativeValue - g.CostBasis
}

func (g *Holding) ProfitPercent() float64 {
	return g.Profit() / g.CostBasis
}

type byCurrency []*coinbase.SpotRate

func (s byCurrency) Len() int {
	return len(s)
}
func (s byCurrency) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byCurrency) Less(i, j int) bool {
	return s[i].Base < s[j].Base
}

func calcHoldings(since time.Time) ([]*Holding, []*coinbase.SpotRate) {
	cb := &coinbase.Client{
		Key:    os.Getenv("COINBASE_KEY"),
		Secret: os.Getenv("COINBASE_SECRET"),
	}
	accounts, err := cb.GetAccounts()
	must(err)
	accountIDs := make([]string, 0)
	for _, a := range accounts {
		accountIDs = append(accountIDs, a.ID)
	}
	trans, err := cb.GetAllTransactions(accountIDs)
	must(err)

	amount := make(map[coinbase.Currency]float64, 0)
	costBasis := make(map[coinbase.Currency]float64, 0)

	for _, t := range trans {
		if t.CreatedAt.Before(since) {
			continue
		}
		switch t.Type {
		case "send":
			amount[t.Amount.Currency] += t.Amount.Amount()
		case "buy":
			amount[t.Amount.Currency] += t.Amount.Amount()
			costBasis[t.Amount.Currency] += t.Buy.Total.Amount()
		case "sell":
			amount[t.Amount.Currency] += t.Amount.Amount()
			costBasis[t.Amount.Currency] -= t.Sell.Total.Amount()
		default:
			panic("Unhandled transaction type: " + t.Type)
		}
	}
	spot, err := cb.GetSpotRates()
	must(err)
	sort.Sort(byCurrency(spot))
	holdings := make([]*Holding, 0)
	for _, s := range spot {
		val := amount[coinbase.Currency(s.Base)]
		if val == 0 {
			continue
		}
		holdings = append(holdings, &Holding{
			Currency:    s.Base,
			Value:       val,
			CostBasis:   costBasis[coinbase.Currency(s.Base)],
			NativeValue: val * s.Amount(),
		})
	}

	return holdings, spot
}

func js(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func maxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func printLine(w io.Writer, v []string) {
	fmt.Fprintln(w, strings.Join(v, "\t")+"\t")
}

func printSep(w io.Writer, v []string) {
	vv := make([]string, 0, len(v))
	for _, col := range v {
		vv = append(vv, strings.Repeat("-", maxInt(len(col), 10)))
	}
	printLine(w, vv)
}
