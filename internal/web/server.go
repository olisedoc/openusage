package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/janekbaraniewski/openusage/internal/core"
	"github.com/janekbaraniewski/openusage/internal/export"
)

//go:embed page.html
var pageFS embed.FS

// Card is one provider tile for the web dashboard.
type Card struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Status   string      `json:"status"`
	Message  string      `json:"message,omitempty"`
	Metrics  []MetricBar `json:"metrics"`
}

// MetricBar is a remaining-quota bar.
type MetricBar struct {
	Label        string   `json:"label"`
	RemainingPct *float64 `json:"remaining_pct,omitempty"`
}

// Payload is GET /api/usage.
type Payload struct {
	FetchedAt time.Time `json:"fetched_at"`
	Cards     []Card    `json:"cards"`
}

var preferred = []string{"opencode", "claude_code", "xai"}

func displayName(providerID string) string {
	switch providerID {
	case "opencode":
		return "OpenCode Go"
	case "claude_code":
		return "Claude Code"
	case "xai":
		return "Grok"
	default:
		return providerID
	}
}

func barsFrom(snap core.UsageSnapshot) []MetricBar {
	keys := make([]string, 0, len(snap.Metrics))
	for k := range snap.Metrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]MetricBar, 0, len(keys))
	for _, k := range keys {
		m := snap.Metrics[k]
		label := k
		if m.Window != "" {
			label = k + " (" + m.Window + ")"
		}
		bar := MetricBar{Label: label}
		pct := m.Percent()
		if pct >= 0 {
			bar.RemainingPct = &pct
		}
		out = append(out, bar)
	}
	return out
}

func cardsFrom(snaps []core.UsageSnapshot) []Card {
	byID := map[string]core.UsageSnapshot{}
	for _, s := range snaps {
		byID[s.ProviderID] = s
	}
	seen := map[string]bool{}
	var cards []Card
	add := func(id string) {
		s, ok := byID[id]
		if !ok {
			return
		}
		seen[id] = true
		cards = append(cards, Card{
			ID:      id,
			Name:    displayName(id),
			Status:  string(s.Status),
			Message: s.Message,
			Metrics: barsFrom(s),
		})
	}
	for _, id := range preferred {
		add(id)
	}
	rest := make([]string, 0)
	for id := range byID {
		if !seen[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	for _, id := range rest {
		add(id)
	}
	return cards
}

// Server serves the usage web UI and JSON API.
type Server struct {
	addr string
	mu   sync.RWMutex
	last Payload
}

func NewServer(addr string) *Server {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = "127.0.0.1:8060"
	}
	return &Server{
		addr: addr,
		last: Payload{
			FetchedAt: time.Time{},
			Cards: []Card{{
				ID:      "loading",
				Name:    "Loading",
				Status:  "UNKNOWN",
				Message: "Collecting snapshots…",
			}},
		},
	}
}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/usage", s.handleAPI)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/", s.handlePage)
	return mux
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := pageFS.ReadFile("page.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	payload := s.last
	s.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) refresh(ctx context.Context) {
	snaps, _, err := export.Collect(ctx, export.SourceDirect)
	payload := Payload{FetchedAt: time.Now().UTC()}
	if err != nil {
		payload.Cards = []Card{{
			ID:      "error",
			Name:    "Collect failed",
			Status:  "ERROR",
			Message: err.Error(),
		}}
	} else {
		payload.Cards = cardsFrom(snaps)
	}
	s.mu.Lock()
	s.last = payload
	s.mu.Unlock()
}

// ListenAndServe serves until ctx is done.
func (s *Server) ListenAndServe(ctx context.Context) error {
	go func() {
		s.refresh(ctx)
		t := time.NewTicker(45 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.refresh(ctx)
			}
		}
	}()
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("web: listen %s: %w", s.addr, err)
	}
	srv := &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		sh, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sh)
		return nil
	case err := <-errCh:
		return err
	}
}
