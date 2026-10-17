package web

import (
	"testing"

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
