package chatgpt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
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

func TestCachedConversationsReturnsStoredVisibleItems(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "conversations.json")
	t.Setenv(conversationCacheEnvVar, cachePath)

	_, err := SaveConversationCache(ConversationCache{
		Conversations: []Conversation{
			{ID: "visible", Title: "Visible"},
			{ID: "hidden", Title: "Hidden"},
		},
		Hidden: map[string]ConversationCacheEntry{
			"hidden": {
				Visibility: ConversationVisibilityDeleted,
				UpdatedAt:  time.Date(2026, time.March, 25, 9, 0, 0, 0, time.UTC),
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConversationCache() error = %v", err)
	}

	client := &Client{}
	got, err := client.CachedConversations()
	if err != nil {
		t.Fatalf("CachedConversations() error = %v", err)
	}

	want := []Conversation{{ID: "visible", Title: "Visible"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CachedConversations() = %#v, want %#v", got, want)
	}
}

func TestListConversationsRefreshesCacheAndFiltersHiddenItems(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "conversations.json")
	t.Setenv(conversationCacheEnvVar, cachePath)

	_, err := SaveConversationCache(ConversationCache{
		Conversations: []Conversation{{ID: "cached-only", Title: "Cached only"}},
		Hidden: map[string]ConversationCacheEntry{
			"archived-1": {
				Visibility: ConversationVisibilityArchived,
				UpdatedAt:  time.Date(2026, time.March, 25, 9, 30, 0, 0, time.UTC),
			},
		},
	})
	if err != nil {
		t.Fatalf("SaveConversationCache() error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/conversations" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		offset := r.URL.Query().Get("offset")
		var payload conversationList
		switch offset {
		case "0":
			payload.Items = []Conversation{
				{ID: "visible-1", Title: "Visible 1"},
				{ID: "archived-1", Title: "Archived locally"},
			}
		case "2":
			payload.Items = []Conversation{
				{ID: "visible-2", Title: "Visible 2"},
			}
		default:
			payload.Items = []Conversation{}
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("json.NewEncoder().Encode() error = %v", err)
		}
	}))
	defer server.Close()

	client := newBackendTestClient(t, server)

	got, err := client.ListConversations(context.Background(), 2)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}

	want := []Conversation{
		{ID: "visible-1", Title: "Visible 1"},
		{ID: "visible-2", Title: "Visible 2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListConversations() = %#v, want %#v", got, want)
	}

	cache, _, err := LoadConversationCache()
	if err != nil {
		t.Fatalf("LoadConversationCache() error = %v", err)
	}
	if !reflect.DeepEqual(cache.Conversations, want) {
		t.Fatalf("cached conversations = %#v, want %#v", cache.Conversations, want)
	}
	if _, ok := cache.Hidden["archived-1"]; !ok {
		t.Fatal("hidden archived conversation was not preserved after refresh")
	}
}

func TestConversationMutationsUpdateLocalCacheVisibility(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		id         string
		visibility ConversationVisibility
		call       func(context.Context, *Client, string) error
	}{
		{
			name:       "archive",
			method:     http.MethodPatch,
			id:         "archive-me",
			visibility: ConversationVisibilityArchived,
			call: func(ctx context.Context, client *Client, id string) error {
				return client.ArchiveConversation(ctx, id)
			},
		},
		{
			name:       "delete",
			method:     http.MethodPatch,
			id:         "delete-me",
			visibility: ConversationVisibilityDeleted,
			call: func(ctx context.Context, client *Client, id string) error {
				return client.DeleteConversation(ctx, id)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cachePath := filepath.Join(t.TempDir(), "conversations.json")
			t.Setenv(conversationCacheEnvVar, cachePath)

			_, err := SaveConversationCache(ConversationCache{
				Conversations: []Conversation{
					{ID: tc.id, Title: "Mutated"},
					{ID: "keep-me", Title: "Keep me"},
				},
			})
			if err != nil {
				t.Fatalf("SaveConversationCache() error = %v", err)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method {
					t.Fatalf("request method = %s, want %s", r.Method, tc.method)
				}
				if r.URL.Path != "/backend-api/conversation/"+tc.id {
					t.Fatalf("request path = %s, want /backend-api/conversation/%s", r.URL.Path, tc.id)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer server.Close()

			client := newBackendTestClient(t, server)
			if err := tc.call(context.Background(), client, tc.id); err != nil {
				t.Fatalf("mutation error = %v", err)
			}

			cache, _, err := LoadConversationCache()
			if err != nil {
				t.Fatalf("LoadConversationCache() error = %v", err)
			}
			if len(cache.Conversations) != 1 || cache.Conversations[0].ID != "keep-me" {
				t.Fatalf("cached conversations after mutation = %#v, want only keep-me", cache.Conversations)
			}
			entry, ok := cache.Hidden[tc.id]
			if !ok {
				t.Fatalf("hidden entry missing for %s", tc.id)
			}
			if entry.Visibility != tc.visibility {
				t.Fatalf("hidden visibility = %q, want %q", entry.Visibility, tc.visibility)
			}
		})
	}
}

func newBackendTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", server.URL, err)
	}

	httpClient := server.Client()
	httpClient.Transport = rewriteRequestTransport{
		baseURL: serverURL,
		next:    http.DefaultTransport,
	}

	return &Client{
		httpClient:  httpClient,
		accessToken: "test-access-token",
	}
}

type rewriteRequestTransport struct {
	baseURL *url.URL
	next    http.RoundTripper
}

func (t rewriteRequestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.baseURL.Scheme
	clone.URL.Host = t.baseURL.Host
	clone.Host = t.baseURL.Host
	return t.next.RoundTrip(clone)
}
