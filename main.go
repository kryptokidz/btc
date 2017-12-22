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

type byCurrency [][]string

func (s byCurrency) Len() int {
	return len(s)
}
func (s byCurrency) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byCurrency) Less(i, j int) bool {
	return s[i][0] < s[j][0]
}

func main() {
	var (
		sinceDate string
		all       bool
	)
	flag.StringVar(&sinceDate, "since", "", "ISO-8601 date")
	flag.BoolVar(&all, "all", false, "show earnings since zero time")

	flag.Parse()

	if sinceDate != "" && all {
		log.Fatal("one of -since or -zero")
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

	gains := calcGains(since)

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight)
	headers := []string{"", "Cost Basis", "Amount", "$", "+/-", "%"}
	printLine(w, headers)
	printSep(w, headers)

	var sums struct {
		costBasis, nativeValue float64
	}
	var output [][]string
	for _, gain := range gains {
		sums.costBasis += gain.CostBasis
		sums.nativeValue += gain.NativeValue
		output = append(output, []string{
			gain.Currency,
			fmtUSD(gain.CostBasis),
			fmtVal(gain.Value),
			fmtUSD(gain.NativeValue),
			sign(gain.Profit()) + fmtUSD(math.Abs(gain.Profit())),
			sign(gain.ProfitPercent()) + fmtPCT(math.Abs(gain.ProfitPercent())),
		})

	}
	sort.Sort(byCurrency(output))
	for _, v := range output {
		printLine(w, v)
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
	return fmt.Sprintf("%.2f%%", val*100)
}

func fmtUSD(val float64) string {
	return fmt.Sprintf("$%.2f", val)
}

func fmtVal(val float64) string {
	return fmt.Sprintf("%.2f", val)
}

type Gains struct {
	Currency    string
	Value       float64
	CostBasis   float64
	NativeValue float64
}

func (g *Gains) Profit() float64 {
	return g.NativeValue - g.CostBasis
}

func (g *Gains) ProfitPercent() float64 {
	return g.Profit() / g.CostBasis
}

func calcGains(since time.Time) []*Gains {
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
	gains := make([]*Gains, 0)
	for _, s := range spot {
		val := amount[coinbase.Currency(s.Base)]
		if val == 0 {
			continue
		}
		gains = append(gains, &Gains{
			Currency:    s.Base,
			Value:       val,
			CostBasis:   costBasis[coinbase.Currency(s.Base)],
			NativeValue: val * s.Amount(),
		})
	}

	return gains
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
		vv = append(vv, strings.Repeat("-", maxInt(len(col), 8)))
	}
	printLine(w, vv)
}
