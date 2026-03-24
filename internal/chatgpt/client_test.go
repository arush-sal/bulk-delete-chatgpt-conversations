package chatgpt

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
)

func TestReadCookieValueSupportsChunkedSessionCookies(t *testing.T) {
	token := "part-0-part-1-part-2"

	got := readCookieValue([]*network.Cookie{
		{Name: sessionCookie + ".1", Value: "part-1-"},
		{Name: csrfCookie, Value: "csrf-token"},
		{Name: sessionCookie + ".0", Value: "part-0-"},
		{Name: sessionCookie + ".2", Value: "part-2"},
	}, sessionCookie)

	if got != token {
		t.Fatalf("readCookieValue(chunked) = %q, want %q", got, token)
	}
}

func TestReadCookieValueReturnsEmptyWhenChunkSequenceHasGap(t *testing.T) {
	got := readCookieValue([]*network.Cookie{
		{Name: sessionCookie + ".0", Value: "part-0-"},
		{Name: sessionCookie + ".2", Value: "part-2"},
	}, sessionCookie)

	if got != "" {
		t.Fatalf("readCookieValue(missing chunk) = %q, want empty string", got)
	}
}

func TestSplitCookieChunksRoundTripsLargeSessionToken(t *testing.T) {
	token := strings.Repeat("a", cookieChunkLen*2+25)

	cookies := splitCookieChunks(sessionCookie, token)
	if len(cookies) != 3 {
		t.Fatalf("splitCookieChunks() count = %d, want 3", len(cookies))
	}
	if cookies[0].Name != sessionCookie+".0" {
		t.Fatalf("first chunk name = %q, want %q", cookies[0].Name, sessionCookie+".0")
	}
	if cookies[1].Name != sessionCookie+".1" {
		t.Fatalf("second chunk name = %q, want %q", cookies[1].Name, sessionCookie+".1")
	}
	if cookies[2].Name != sessionCookie+".2" {
		t.Fatalf("third chunk name = %q, want %q", cookies[2].Name, sessionCookie+".2")
	}

	var browserCookies []*network.Cookie
	for _, cookie := range cookies {
		browserCookies = append(browserCookies, &network.Cookie{
			Name:  cookie.Name,
			Value: cookie.Value,
		})
	}

	if got := readCookieValue(browserCookies, sessionCookie); got != token {
		t.Fatalf("readCookieValue(round trip) = %d bytes, want %d bytes", len(got), len(token))
	}
}

func TestInitializeHTTPClientSeedsChunkedSessionCookies(t *testing.T) {
	token := strings.Repeat("b", cookieChunkLen+10)

	client, err := New(Config{SessionToken: token})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := client.initializeHTTPClient(); err != nil {
		t.Fatalf("initializeHTTPClient() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, chatGPTBaseURL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}

	cookies := client.httpClient.Jar.Cookies(req.URL)
	if len(cookies) != 2 {
		t.Fatalf("jar cookie count = %d, want 2", len(cookies))
	}
	if cookies[0].Name != sessionCookie+".0" {
		t.Fatalf("first jar cookie name = %q, want %q", cookies[0].Name, sessionCookie+".0")
	}
	if cookies[1].Name != sessionCookie+".1" {
		t.Fatalf("second jar cookie name = %q, want %q", cookies[1].Name, sessionCookie+".1")
	}
}

func TestBrowserCommandContextUsesBrowserContextAndPreservesDeadline(t *testing.T) {
	type ctxKey string

	sourceCtx, sourceCancel := context.WithTimeout(context.WithValue(context.Background(), ctxKey("source"), "source"), 5*time.Minute)
	defer sourceCancel()

	browserCtx := context.WithValue(context.Background(), ctxKey("browser"), "browser")
	client := &Client{browserCtx: browserCtx}

	gotCtx, cancel := client.browserCommandContext(sourceCtx)
	defer cancel()

	if got := gotCtx.Value(ctxKey("browser")); got != "browser" {
		t.Fatalf("browserCommandContext() browser value = %v, want browser context value", got)
	}
	if got := gotCtx.Value(ctxKey("source")); got != nil {
		t.Fatalf("browserCommandContext() source value = %v, want nil because browser context should be used", got)
	}

	gotDeadline, ok := gotCtx.Deadline()
	if !ok {
		t.Fatalf("browserCommandContext() deadline missing")
	}
	wantDeadline, _ := sourceCtx.Deadline()
	if !gotDeadline.Equal(wantDeadline) {
		t.Fatalf("browserCommandContext() deadline = %v, want %v", gotDeadline, wantDeadline)
	}
}
