// DOSRouter CLI - Smart LLM Router
//
// Usage:
//
//	dosrouter serve --port 8080 --upstream https://api.example.com --api-key sk-xxx
//	dosrouter classify "Write a Go middleware with rate limiting"
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/partners"
	"github.com/DOS/DOSRouter/proxy"
	"github.com/DOS/DOSRouter/router"
	"github.com/DOS/DOSRouter/stats"
	"github.com/DOS/DOSRouter/wallet"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe()
	case "classify":
		cmdClassify()
	case "models":
		cmdModels()
	case "stats":
		cmdStats()
	case "logs":
		cmdLogs()
	case "partners":
		cmdPartners()
	case "cache":
		cmdCache()
	case "report":
		cmdReport()
	case "wallet":
		cmdWallet()
	case "chain":
		cmdChain()
	case "doctor":
		cmdDoctor()
	case "version":
		fmt.Printf("DOSRouter %s (Go port of ClawRouter)\n", proxy.Version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`DOSRouter - Smart LLM Router

Usage:
  dosrouter serve              Start the proxy server
  dosrouter classify "prompt"  Classify a prompt's complexity
  dosrouter models             List available models with pricing
  dosrouter stats [--days N]   Usage statistics (default: 7 days)
  dosrouter stats clear        Clear all usage logs
  dosrouter logs [--days N]    Per-request log (default: 1 day)
  dosrouter cache              Cache statistics
  dosrouter report [period]    Usage report (daily, weekly, monthly)
  dosrouter partners           List available partner APIs
  dosrouter wallet             Show wallet address and balance
  dosrouter wallet recover     Recover wallet from mnemonic
  dosrouter chain [name]       Show or switch payment chain
  dosrouter doctor             AI-powered diagnostics
  dosrouter version            Show version

Serve flags:
  --port PORT          Listen port (default: 8080)
  --upstream URL       Upstream API base URL
  --api-key KEY        API key for upstream

Environment:
  DOSROUTER_UPSTREAM       Upstream API base URL
  DOSROUTER_API_KEY        API key for upstream
  DOSROUTER_WALLET_KEY     Private key (hex) for wallet
  DOSROUTER_CHAIN          Payment chain (default: doschain)
  DOSROUTER_RPC_URL        RPC endpoint for payment chain`)
}

func cmdServe() {
	port := 8080
	upstream := ""
	apiKey := ""

	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--port":
			if i+1 < len(os.Args) {
				p, err := strconv.Atoi(os.Args[i+1])
				if err == nil {
					port = p
				}
				i++
			}
		case "--upstream":
			if i+1 < len(os.Args) {
				upstream = os.Args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(os.Args) {
				apiKey = os.Args[i+1]
				i++
			}
		}
	}

	// Also check env vars
	if upstream == "" {
		upstream = os.Getenv("DOSROUTER_UPSTREAM")
	}
	if apiKey == "" {
		apiKey = os.Getenv("DOSROUTER_API_KEY")
	}

	if upstream == "" {
		fmt.Fprintln(os.Stderr, "Error: --upstream or DOSROUTER_UPSTREAM is required")
		os.Exit(1)
	}

	srv := proxy.New(proxy.Config{
		Port:           port,
		UpstreamBase:   upstream,
		UpstreamAPIKey: apiKey,
	})

	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func cmdClassify() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: dosrouter classify \"your prompt here\"")
		os.Exit(1)
	}

	prompt := strings.Join(os.Args[2:], " ")
	config := router.DefaultRoutingConfig()
	pricingMap := models.BuildPricingMap()

	fullText := prompt
	estimatedTokens := int(math.Ceil(float64(len(fullText)) / 4))

	scoring := router.ClassifyByRules(prompt, "", estimatedTokens, config.Scoring)

	decision, _ := router.Route(prompt, "", 4096, router.RouterOptions{
		Config:         config,
		ModelPricing:   pricingMap,
		RoutingProfile: "auto",
	})

	output := map[string]interface{}{
		"scoring":  scoring,
		"decision": decision,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(output)
}

func cmdModels() {
	for _, m := range models.Models {
		if m.Deprecated {
			continue
		}
		pricing := "FREE"
		if m.InputPrice > 0 || m.OutputPrice > 0 {
			pricing = fmt.Sprintf("$%.2f/$%.2f per 1M tokens", m.InputPrice, m.OutputPrice)
		}
		tags := []string{}
		if m.Reasoning {
			tags = append(tags, "reasoning")
		}
		if m.Vision {
			tags = append(tags, "vision")
		}
		if m.Agentic {
			tags = append(tags, "agentic")
		}
		if m.ToolCalling {
			tags = append(tags, "tools")
		}
		tagStr := ""
		if len(tags) > 0 {
			tagStr = " [" + strings.Join(tags, ", ") + "]"
		}
		fmt.Printf("  %-40s %s%s\n", m.ID, pricing, tagStr)
	}
}

func cmdStats() {
	// Handle "stats clear" subcommand
	if len(os.Args) > 2 && (os.Args[2] == "clear" || os.Args[2] == "reset") {
		if err := stats.ClearStats(); err != nil {
			fmt.Fprintf(os.Stderr, "Error clearing stats: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Usage statistics cleared.")
		return
	}

	days := 7
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--days" && i+1 < len(os.Args) {
			if d, err := strconv.Atoi(os.Args[i+1]); err == nil {
				days = d
			}
			i++
		}
	}
	s := stats.GetStats(days)
	fmt.Println(stats.FormatStatsASCII(s))
}

func cmdLogs() {
	days := 1
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--days" && i+1 < len(os.Args) {
			if d, err := strconv.Atoi(os.Args[i+1]); err == nil {
				days = d
			}
			i++
		}
	}
	fmt.Println(stats.FormatRecentLogs(days, 20))
}

func cmdPartners() {
	services := partners.PartnerServices
	if len(services) == 0 {
		fmt.Println("No partner APIs available.")
		return
	}

	fmt.Printf("\nDOSRouter Partner APIs (%d services)\n\n", len(services))
	for _, svc := range services {
		fmt.Printf("  %s\n", svc.Name)
		fmt.Printf("    %s\n", svc.Description)
		fmt.Printf("    Tool:    dosrouter_%s\n", svc.ID)
		fmt.Printf("    Method:  %s %s\n", svc.Method, svc.BaseURL)
		fmt.Println()
	}
}

func cmdCache() {
	// Query the running proxy's /cache endpoint
	port := os.Getenv("DOSROUTER_PORT")
	if port == "" {
		port = "8080"
	}
	resp, err := httpGet(fmt.Sprintf("http://localhost:%s/cache", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: proxy not running on port %s (%v)\n", port, err)
		os.Exit(1)
	}
	var cacheStats struct {
		Hits      int64   `json:"hits"`
		Misses    int64   `json:"misses"`
		Evictions int64   `json:"evictions"`
		HitRate   float64 `json:"hitRate"`
	}
	if json.Unmarshal(resp, &cacheStats) != nil {
		fmt.Println(string(resp))
		return
	}
	fmt.Println("+----------------------------------+")
	fmt.Println("| Response Cache                   |")
	fmt.Println("+----------------------------------+")
	fmt.Printf("| Hits:      %-21d|\n", cacheStats.Hits)
	fmt.Printf("| Misses:    %-21d|\n", cacheStats.Misses)
	fmt.Printf("| Evictions: %-21d|\n", cacheStats.Evictions)
	fmt.Printf("| Hit Rate:  %-20.1f%%|\n", cacheStats.HitRate*100)
	fmt.Println("+----------------------------------+")
}

func cmdReport() {
	period := "daily"
	jsonOutput := false
	for i := 2; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "daily", "weekly", "monthly":
			period = os.Args[i]
		case "--json":
			jsonOutput = true
		}
	}

	days := 1
	switch period {
	case "weekly":
		days = 7
	case "monthly":
		days = 30
	}

	s := stats.GetStats(days)
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(s)
		return
	}
	fmt.Printf("DOSRouter Usage Report (%s)\n", period)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println(stats.FormatStatsASCII(s))
}

func cmdWallet() {
	if len(os.Args) > 2 && os.Args[2] == "recover" {
		cmdWalletRecover()
		return
	}

	w, err := wallet.LoadOrCreate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading wallet: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("+--------------------------------------------------+")
	fmt.Println("| DOSRouter Wallet                                 |")
	fmt.Println("+--------------------------------------------------+")
	fmt.Printf("| Address:  %-38s|\n", w.Address())
	fmt.Printf("| Chain:    %-38s|\n", w.Chain())

	balance, err := w.GetBalance()
	if err != nil {
		fmt.Printf("| Balance:  %-38s|\n", "error: "+err.Error())
	} else {
		fmt.Printf("| Balance:  $%-37.6f|\n", balance)
	}
	fmt.Println("+--------------------------------------------------+")

	if w.IsNew() {
		fmt.Println("\nNew wallet created. Fund it with USDC on", w.Chain())
		fmt.Println("Mnemonic (save this!):", w.Mnemonic())
	}
}

func cmdWalletRecover() {
	fmt.Print("Enter mnemonic phrase: ")
	var mnemonic string
	fmt.Scanln(&mnemonic)
	// Read full line (mnemonic has spaces)
	if mnemonic == "" {
		fmt.Fprintln(os.Stderr, "Error: mnemonic required")
		os.Exit(1)
	}

	w, err := wallet.Recover(mnemonic)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error recovering wallet: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wallet recovered: %s\n", w.Address())
}

func cmdChain() {
	w, err := wallet.LoadOrCreate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading wallet: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 2 {
		chain := os.Args[2]
		if err := w.SetChain(chain); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Payment chain set to: %s\n", chain)
		return
	}

	fmt.Printf("Current chain: %s\n", w.Chain())
	fmt.Println("\nAvailable chains:")
	for _, c := range wallet.SupportedChains() {
		marker := "  "
		if c == w.Chain() {
			marker = "* "
		}
		fmt.Printf("  %s%s\n", marker, c)
	}
}

func cmdDoctor() {
	fmt.Println("DOSRouter Diagnostics")
	fmt.Println(strings.Repeat("=", 50))

	// 1. Version
	fmt.Printf("\n[Version] %s\n", proxy.Version)

	// 2. Wallet
	fmt.Print("\n[Wallet] ")
	w, err := wallet.LoadOrCreate()
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
	} else {
		fmt.Printf("%s on %s\n", w.Address(), w.Chain())
		balance, err := w.GetBalance()
		if err != nil {
			fmt.Printf("  Balance: error (%v)\n", err)
		} else {
			fmt.Printf("  Balance: $%.6f\n", balance)
			if balance < 1.0 {
				fmt.Println("  WARNING: Low balance (< $1.00)")
			}
		}
	}

	// 3. Proxy
	fmt.Print("\n[Proxy] ")
	port := os.Getenv("DOSROUTER_PORT")
	if port == "" {
		port = "8080"
	}
	healthResp, err := httpGet(fmt.Sprintf("http://localhost:%s/health?full=true", port))
	if err != nil {
		fmt.Printf("NOT RUNNING on port %s\n", port)
	} else {
		var health map[string]interface{}
		json.Unmarshal(healthResp, &health)
		fmt.Printf("Running on port %s (status: %v)\n", port, health["status"])
		if sessions, ok := health["sessions"].(float64); ok {
			fmt.Printf("  Sessions: %.0f\n", sessions)
		}
	}

	// 4. Upstream
	fmt.Print("\n[Upstream] ")
	upstream := os.Getenv("DOSROUTER_UPSTREAM")
	if upstream == "" {
		fmt.Println("NOT CONFIGURED (set DOSROUTER_UPSTREAM)")
	} else {
		_, err := httpGet(upstream + "/v1/models")
		if err != nil {
			fmt.Printf("UNREACHABLE (%s)\n", upstream)
		} else {
			fmt.Printf("OK (%s)\n", upstream)
		}
	}

	// 5. Usage (last 24h)
	fmt.Println("\n[Usage - Last 24h]")
	s := stats.GetStats(1)
	fmt.Printf("  Requests: %d\n", s.TotalRequests)
	fmt.Printf("  Cost:     $%.4f\n", s.TotalCost)
	if s.TotalSavings > 0 {
		fmt.Printf("  Savings:  $%.4f (%.1f%%)\n", s.TotalSavings, s.SavingsPercentage)
	}

	// 6. Models
	fmt.Printf("\n[Models] %d available\n", countActiveModels())

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("Diagnostics complete.")
}

func countActiveModels() int {
	count := 0
	for _, m := range models.Models {
		if !m.Deprecated {
			count++
		}
	}
	return count
}

func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
