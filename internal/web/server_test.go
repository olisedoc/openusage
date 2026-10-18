package web

import (
	"context"
	"testing"
	"time"

	"github.com/janekbaraniewski/openusage/internal/core"
)

func TestCardsFromPreferredOrder(t *testing.T) {
	snaps := []core.UsageSnapshot{
		{ProviderID: "xai", Status: core.StatusOK, Message: "grok"},
		{ProviderID: "openrouter", Status: core.StatusOK},
		{ProviderID: "opencode", Status: core.StatusOK},
		{ProviderID: "claude_code", Status: core.StatusOK},
	}
	cards := cardsFrom(snaps)
	if len(cards) != 4 {
		t.Fatalf("len=%d", len(cards))
	}
	want := []string{"opencode", "claude_code", "xai", "openrouter"}
	for i, id := range want {
		if cards[i].ID != id {
			t.Fatalf("cards[%d].ID=%q want %q", i, cards[i].ID, id)
		}
	}
	if cards[0].Name != "OpenCode Go" || cards[2].Name != "Grok" {
		t.Fatalf("names: %+v", cards)
	}
}

// TestBarsFromKeepsOnlyQuota checks that counters and estimates (no reset) do
// not render as empty quota bars.
func TestBarsFromKeepsOnlyQuota(t *testing.T) {
	snap := core.UsageSnapshot{
		ProviderID: "claude_code",
		Metrics: map[string]core.Metric{
			"usage_five_hour":    {Used: core.Float64Ptr(2), Limit: core.Float64Ptr(100), Window: "5h"},
			"cache_hit_ratio":    {Used: core.Float64Ptr(97), Limit: core.Float64Ptr(100)},
			"7d_input_tokens":    {Used: core.Float64Ptr(1000)},
			"ai_code_percentage": {Used: core.Float64Ptr(100), Limit: core.Float64Ptr(100)},
		},
		Resets: map[string]time.Time{"usage_five_hour": time.Now().Add(time.Hour)},
	}
	bars := barsFrom(snap)
	if len(bars) != 1 {
		t.Fatalf("bars=%+v", bars)
	}
	if bars[0].Label != "usage_five_hour (5h)" || bars[0].RemainingPct == nil || *bars[0].RemainingPct != 98 {
		t.Fatalf("bar=%+v", bars[0])
	}
}

func TestRefreshPublishesSnapshot(t *testing.T) {
	s := NewServer("127.0.0.1:0")
	s.collect = func(context.Context) ([]core.UsageSnapshot, error) {
		return []core.UsageSnapshot{{
			ProviderID: "codex",
			Status:     core.StatusOK,
			Metrics: map[string]core.Metric{
				"rate_limit_primary": {Used: core.Float64Ptr(40), Limit: core.Float64Ptr(100), Window: "30d"},
			},
			Resets: map[string]time.Time{"rate_limit_primary": time.Now().Add(time.Hour)},
		}}, nil
	}
	s.refresh(context.Background())
	if s.last.FetchedAt.IsZero() {
		t.Fatal("fetched_at not set")
	}
	if len(s.last.Cards) != 1 || s.last.Cards[0].ID != "codex" || len(s.last.Cards[0].Metrics) != 1 {
		t.Fatalf("cards=%+v", s.last.Cards)
	}
	if pct := s.last.Cards[0].Metrics[0].RemainingPct; pct == nil || *pct != 60 {
		t.Fatalf("metrics=%+v", s.last.Cards[0].Metrics)
	}
}

// TestRefreshSurvivesWedgedCollect guards the dashboard against a provider that
// never returns: the payload must switch to an explicit timeout card instead of
// staying on the loading placeholder.
func TestRefreshSurvivesWedgedCollect(t *testing.T) {
	s := NewServer("127.0.0.1:0")
	s.budget = 50 * time.Millisecond
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	s.collect = func(context.Context) ([]core.UsageSnapshot, error) {
		<-release
		return nil, nil
	}
	start := time.Now()
	s.refresh(context.Background())
	if elapsed := time.Since(start); elapsed > refreshGrace+5*time.Second {
		t.Fatalf("refresh blocked for %s", elapsed)
	}
	if s.last.FetchedAt.IsZero() {
		t.Fatal("fetched_at not set")
	}
	if len(s.last.Cards) != 1 || s.last.Cards[0].ID != "timeout" || s.last.Cards[0].Status != "ERROR" {
		t.Fatalf("cards=%+v", s.last.Cards)
	}
}
