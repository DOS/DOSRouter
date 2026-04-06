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
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/proxy"
	"github.com/DOS/DOSRouter/router"
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
	case "version":
		fmt.Println("DOSRouter v1.0.0 (ported from ClawRouter)")
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`DOSRouter - Smart LLM Router

Usage:
  dosrouter serve     Start the proxy server
  dosrouter classify  Classify a prompt's complexity
  dosrouter models    List available models
  dosrouter version   Show version

Serve flags:
  --port PORT          Listen port (default: 8080)
  --upstream URL       Upstream API base URL
  --api-key KEY        API key for upstream

Classify:
  dosrouter classify "your prompt here"`)
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
