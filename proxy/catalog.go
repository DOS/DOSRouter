package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/DOS/DOSRouter/models"
	"github.com/DOS/DOSRouter/router"
)

type catalogSnapshot struct {
	models  []models.ModelDef
	pricing map[string]router.ModelPricing
}

func (s *Server) catalogView() catalogSnapshot {
	s.catalogMu.RLock()
	defer s.catalogMu.RUnlock()
	if s.catalog != nil {
		return *s.catalog
	}
	return catalogSnapshot{models.Models, s.modelPricing}
}

func (c catalogSnapshot) model(id string) *models.ModelDef {
	for i := range c.models {
		if c.models[i].ID == id {
			return &c.models[i]
		}
	}
	return nil
}

type catalogRow struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Categories []string `json:"categories"`
	Available  *bool    `json:"available"`
	Billing    string   `json:"billing_mode"`
	Pricing    struct {
		Input  *float64 `json:"input"`
		Output *float64 `json:"output"`
		Flat   *float64 `json:"flat"`
	} `json:"pricing"`
	Context   *int `json:"context_window"`
	MaxOutput *int `json:"max_output"`
}

var catalogID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+/-]*$`)

func validPrice(p *float64) bool { return p != nil && *p >= 0 && !math.IsNaN(*p) && !math.IsInf(*p, 0) }

// projectCatalog changes metadata only; aliases, routing policy and proven tools stay local.
func projectCatalog(body []byte) (catalogSnapshot, error) {
	var payload struct {
		Data []catalogRow `json:"data"`
	}
	invalid := errors.New("invalid gateway catalog")
	if json.Unmarshal(body, &payload) != nil || len(payload.Data) == 0 {
		return catalogSnapshot{}, invalid
	}
	next := catalogSnapshot{models: append([]models.ModelDef(nil), models.Models...), pricing: models.BuildPricingMap()}
	byUpstream := map[string]int{}
	for i, m := range next.models {
		id := m.ID
		if strings.HasPrefix(id, "free/") {
			id = "nvidia/" + strings.TrimPrefix(id, "free/")
		}
		if m.ID == "free/north-mini-code" {
			id = "cohere/north-mini-code"
		}
		if m.ID == "free/laguna-xs-2.1" {
			id = "poolside/laguna-xs-2.1"
		}
		byUpstream[id] = i
	}
	seen := map[string]bool{}
	for _, row := range payload.Data {
		if !catalogID.MatchString(row.ID) || seen[row.ID] || row.Name == "" || row.Categories == nil {
			return catalogSnapshot{}, invalid
		}
		seen[row.ID] = true
		if row.Context != nil && *row.Context <= 0 || row.MaxOutput != nil && *row.MaxOutput <= 0 {
			return catalogSnapshot{}, invalid
		}
		p := router.ModelPricing{}
		switch row.Billing {
		case "paid":
			if !validPrice(row.Pricing.Input) || !validPrice(row.Pricing.Output) {
				return catalogSnapshot{}, invalid
			}
			p.InputPrice = *row.Pricing.Input
			p.OutputPrice = *row.Pricing.Output
		case "free":
			if row.Pricing.Input != nil && *row.Pricing.Input != 0 || row.Pricing.Output != nil && *row.Pricing.Output != 0 {
				return catalogSnapshot{}, invalid
			}
		case "flat":
			if !validPrice(row.Pricing.Flat) {
				return catalogSnapshot{}, invalid
			}
			p.FlatPrice = row.Pricing.Flat
		default:
			continue
		}
		if !slices.Contains(row.Categories, "chat") || row.Available != nil && !*row.Available || !strings.Contains(row.ID, "/") || strings.HasPrefix(row.ID, "blockrun/") {
			continue
		}
		idx, known := byUpstream[row.ID]
		var m models.ModelDef
		if known {
			m = next.models[idx]
			if strings.HasPrefix(m.ID, "free/") && row.Billing != "free" {
				return catalogSnapshot{}, errors.New("gateway changed a free model's billing")
			}
		} else {
			if _, alias := models.ModelAliases[row.ID]; alias {
				continue
			}
			// No remote source may mint a DOSRouter free-product name.
			if strings.HasPrefix(row.ID, "free/") || strings.HasPrefix(row.ID, "dosrouter/") {
				continue
			}
			m = models.ModelDef{ID: row.ID, ContextWindow: 32768, MaxOutput: 4096, Reasoning: slices.Contains(row.Categories, "reasoning"), Vision: slices.Contains(row.Categories, "vision")}
		}
		m.Name = row.Name
		m.InputPrice = p.InputPrice
		m.OutputPrice = p.OutputPrice
		m.Promo = nil
		if row.Context != nil {
			m.ContextWindow = *row.Context
		}
		if row.MaxOutput != nil {
			m.MaxOutput = *row.MaxOutput
		}
		if known {
			next.models[idx] = m
		} else {
			byUpstream[row.ID] = len(next.models)
			next.models = append(next.models, m)
		}
		next.pricing[m.ID] = p
	}
	return next, nil
}

func (s *Server) refreshCatalog(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.config.UpstreamBase, "/")+"/v1/models?format=json", nil)
	if err != nil {
		return err
	}
	if s.config.UpstreamAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.config.UpstreamAPIKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("gateway catalog unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil || len(body) > 4*1024*1024 {
		return errors.New("invalid gateway catalog body")
	}
	next, err := projectCatalog(body)
	if err != nil {
		return err
	}
	s.catalogMu.Lock()
	s.catalog = &next
	s.catalogMu.Unlock()
	return nil
}

func (s *Server) startCatalogRefresh() {
	s.catalogStart.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			failed := false
			for {
				if s.catalogCtx.Err() != nil {
					return
				}
				err := s.refreshCatalog(s.catalogCtx)
				if err != nil && !failed && s.catalogCtx.Err() == nil {
					log.Print("DOSRouter: model catalog refresh failed; keeping the last good snapshot")
				}
				failed = err != nil
				select {
				case <-s.catalogCtx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}
