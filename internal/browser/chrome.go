package browser

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	cdpstorage "github.com/chromedp/cdproto/storage"
	"github.com/chromedp/chromedp"
)

const defaultLoginURL = "https://www.vagaro.com/login"

var errChromeNotFound = errors.New("chrome or chromium executable not found")

// ChromeOptions configures the Chrome-backed authentication flow.
type ChromeOptions struct {
	ExecutablePath string
	Timeout        time.Duration
}

// ChromeBackend captures Vagaro authentication state from Chrome or Chromium.
type ChromeBackend struct {
	executablePath string
	timeout        time.Duration
}

// NewChromeBackend constructs a Chrome-backed browser authenticator.
func NewChromeBackend(opts ChromeOptions) (*ChromeBackend, error) {
	executablePath := opts.ExecutablePath
	if executablePath == "" {
		var err error
		executablePath, err = detectChromeExecutable()
		if err != nil {
			return nil, err
		}
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	return &ChromeBackend{
		executablePath: executablePath,
		timeout:        timeout,
	}, nil
}

// Authenticate opens Chrome, waits for a Vagaro session, and returns the captured bundle.
func (b *ChromeBackend) Authenticate(ctx context.Context, loginURL string) (storage.AuthBundle, error) {
	if loginURL == "" {
		loginURL = defaultLoginURL
	}

	authCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	profileDir, err := os.MkdirTemp("", "vagaro-sync-chrome-*")
	if err != nil {
		return storage.AuthBundle{}, fmt.Errorf("create temporary Chrome profile: %w", err)
	}
	defer func() {
		_ = os.RemoveAll(profileDir)
	}()

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(authCtx, append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(b.executablePath),
		chromedp.UserDataDir(profileDir),
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", false),
		chromedp.Flag("disable-popup-blocking", false),
	)...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	if err := chromedp.Run(browserCtx, network.Enable(), chromedp.Navigate(loginURL)); err != nil {
		return storage.AuthBundle{}, fmt.Errorf("open login page: %w", err)
	}

	var userAgent string
	if err := chromedp.Run(browserCtx, chromedp.Evaluate(`navigator.userAgent`, &userAgent)); err != nil {
		return storage.AuthBundle{}, fmt.Errorf("read browser user agent: %w", err)
	}

	bundle, err := b.waitForAuthenticatedSession(browserCtx)
	if err != nil {
		return storage.AuthBundle{}, err
	}

	bundle.UserAgent = userAgent
	return bundle, nil
}

func (b *ChromeBackend) waitForAuthenticatedSession(ctx context.Context) (storage.AuthBundle, error) {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for {
		bundle, bundleErr := currentAuthBundle(ctx)
		if bundleErr == nil && authSessionReady(bundle) {
			return bundle, nil
		}

		select {
		case <-ctx.Done():
			return storage.AuthBundle{}, fmt.Errorf("wait for authenticated session: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func currentAuthBundle(ctx context.Context) (storage.AuthBundle, error) {
	browserCtx, err := browserExecutorContext(ctx)
	if err != nil {
		return storage.AuthBundle{}, err
	}

	cookies, err := cdpstorage.GetCookies().Do(browserCtx)
	if err != nil {
		return storage.AuthBundle{}, err
	}

	bundle := authBundleFromCookies(cookies)
	bundle.CapturedAt = time.Now().UTC()
	return bundle, nil
}

func browserExecutorContext(ctx context.Context) (context.Context, error) {
	chromeCtx := chromedp.FromContext(ctx)
	if chromeCtx == nil || chromeCtx.Browser == nil {
		return nil, fmt.Errorf("resolve browser executor: invalid chromedp context")
	}

	return cdp.WithExecutor(ctx, chromeCtx.Browser), nil
}

func authBundleFromCookies(cookies []*network.Cookie) storage.AuthBundle {
	bundle := storage.AuthBundle{}

	for _, cookie := range cookies {
		if cookie.Name == "s_utkn" {
			bundle.SUToken = cookie.Value
		}
	}

	return bundle
}

func authSessionReady(bundle storage.AuthBundle) bool {
	return bundle.SUToken != ""
}

func detectChromeExecutable() (string, error) {
	homeDir := ""
	currentUser, err := user.Current()
	if err == nil {
		homeDir = currentUser.HomeDir
	}

	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		filepath.Join(homeDir, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		filepath.Join(homeDir, "Applications/Chromium.app/Contents/MacOS/Chromium"),
	}

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}

		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", errChromeNotFound
}
