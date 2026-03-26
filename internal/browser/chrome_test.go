package browser

import (
	"context"
	"strings"
	"testing"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/chromedp/cdproto/network"
)

func TestAuthSessionReadyWithSUToken(t *testing.T) {
	t.Parallel()

	if !authSessionReady(storage.AuthBundle{SUToken: "token"}) {
		t.Fatal("expected session with s_utkn to be ready")
	}
}

func TestAuthSessionReadyWithoutSUToken(t *testing.T) {
	t.Parallel()

	if authSessionReady(storage.AuthBundle{}) {
		t.Fatal("did not expect session without s_utkn to be ready")
	}
}

func TestCurrentAuthBundleCapturesSUToken(t *testing.T) {
	t.Parallel()

	bundle := authBundleFromCookies([]*network.Cookie{
		{Name: "WebsiteBuilder", Value: `{"EncUid":"encoded-user-id"}`},
		{Name: "s_utkn", Value: "token"},
	})

	if bundle.SUToken != "token" {
		t.Fatalf("SUToken = %q, want %q", bundle.SUToken, "token")
	}
}

func TestBrowserExecutorContextRejectsInvalidChromedpContext(t *testing.T) {
	t.Parallel()

	_, err := browserExecutorContext(context.Background())
	if err == nil {
		t.Fatal("expected invalid chromedp context error")
	}
	if !strings.Contains(err.Error(), "invalid chromedp context") {
		t.Fatalf("error = %v", err)
	}
}
