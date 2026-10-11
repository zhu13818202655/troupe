package oauthserver

import (
	"context"
	"testing"

	"github.com/zhu13818202655/troupe/pkg/store"
)

func TestMatchRedirectURIAcceptsCLILoopbackPort(t *testing.T) {
	client := cliClient()
	got := matchRedirectURI(client, "http://127.0.0.1:54321/oauth/callback")
	if got != "http://127.0.0.1:54321/oauth/callback" {
		t.Fatalf("loopback redirect = %q", got)
	}
}

func TestMatchRedirectURIRejectsNonLoopbackCLIRedirect(t *testing.T) {
	client := cliClient()
	for _, raw := range []string{
		"http://example.com/oauth/callback",
		"https://127.0.0.1:54321/oauth/callback",
		"http://127.0.0.1:54321/elsewhere",
		"http://127.0.0.1/oauth/callback",
		"http://127.0.0.1:0/oauth/callback",
		"http://127.0.0.1:54321/oauth/callback?next=https://evil.example",
	} {
		if got := matchRedirectURI(client, raw); got != "" {
			t.Fatalf("accepted %q as %q", raw, got)
		}
	}
}

func TestMatchRedirectURIKeepsExactRegisteredURIs(t *testing.T) {
	client := &store.OAuthClient{
		ClientID:     "registered",
		RedirectURIs: []string{"https://app.example/callback"},
	}
	if got := matchRedirectURI(client, "https://app.example/callback"); got != "https://app.example/callback" {
		t.Fatalf("registered redirect = %q", got)
	}
	if got := matchRedirectURI(client, "http://127.0.0.1:54321/oauth/callback"); got != "" {
		t.Fatalf("non-CLI client accepted loopback: %q", got)
	}
}

func TestFirstPartyPublicClients(t *testing.T) {
	if !isFirstPartyPublicClient(cliClient()) || !isFirstPartyPublicClient(chromeExtensionClient()) {
		t.Fatal("expected CLI and Chrome extension to be first-party public clients")
	}
	if got := cliClient().Scopes; len(got) != 1 || got[0] != ScopeChat {
		t.Fatalf("CLI granted scopes = %#v, want [%q]", got, ScopeChat)
	}
	if isFirstPartyPublicClient(&store.OAuthClient{ClientID: "ast_other"}) {
		t.Fatal("registered clients must not inherit first-party tenant binding")
	}
}

func TestLegacyFirstPartyClientIDsResolveToRenamedClients(t *testing.T) {
	s := &Server{}
	// Old extension builds sending the legacy client ID must still authenticate.
	ext, err := s.client(context.Background(), LegacyChromeExtensionClientID)
	if err != nil || ext == nil {
		t.Fatalf("legacy chrome extension client lookup failed: %v", err)
	}
	if ext.ClientID != ChromeExtensionClientID {
		t.Fatalf("legacy extension lookup issued client ID %q, want new %q", ext.ClientID, ChromeExtensionClientID)
	}
	if !isFirstPartyPublicClient(ext) {
		t.Fatal("legacy extension client must be treated as first-party public")
	}
	cli, err := s.client(context.Background(), LegacyCLIClientID)
	if err != nil || cli == nil {
		t.Fatalf("legacy CLI client lookup failed: %v", err)
	}
	if cli.ClientID != CLIClientID {
		t.Fatalf("legacy CLI lookup issued client ID %q, want new %q", cli.ClientID, CLIClientID)
	}
	if !isFirstPartyPublicClient(&store.OAuthClient{ClientID: LegacyCLIClientID}) ||
		!isFirstPartyPublicClient(&store.OAuthClient{ClientID: LegacyChromeExtensionClientID}) {
		t.Fatal("legacy client IDs must satisfy first-party public client checks")
	}
}
