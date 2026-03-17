package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	chatGPTBaseURL = "https://chatgpt.com"
	backendAPIURL  = chatGPTBaseURL + "/backend-api"
	sessionAPIURL  = chatGPTBaseURL + "/api/auth/session"
)

type Conversation struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	CreateTime    chatTime `json:"create_time"`
	UpdateTime    chatTime `json:"update_time"`
	IsArchived    bool     `json:"is_archived"`
	IsVisible     bool     `json:"is_visible"`
	CurrentNode   string   `json:"current_node"`
	ConversationT string   `json:"conversation_template_id"`
}

type conversationList struct {
	Items []Conversation `json:"items"`
}

type sessionResponse struct {
	AccessToken string `json:"accessToken"`
}

type Client struct {
	debug        bool
	logger       *log.Logger
	sessionToken string
	csrfToken    string
	accessToken  string
	status       string
	httpClient   *http.Client

	allocCtx    context.Context
	allocCancel context.CancelFunc
	browserCtx  context.Context
	browserStop context.CancelFunc
	profileDir  string

	mu       sync.Mutex
	statusMu sync.RWMutex
}

type AuthError struct {
	StatusCode int
	Message    string
}

func (e *AuthError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("authentication failed with status %d", e.StatusCode)
}

type browserResponse struct {
	Status int             `json:"status"`
	Text   string          `json:"text"`
	Body   json.RawMessage `json:"body"`
}

func NewFromEnv() (*Client, error) {
	sessionToken := strings.TrimSpace(os.Getenv("CHATGPT_SESSION_TOKEN"))
	if sessionToken == "" {
		return nil, errors.New("CHATGPT_SESSION_TOKEN is required")
	}

	debug := parseBoolEnv("DEBUG")
	client := &Client{
		debug:        debug,
		logger:       log.New(os.Stderr, "debug: ", log.LstdFlags),
		sessionToken: sessionToken,
		csrfToken:    strings.TrimSpace(os.Getenv("CHATGPT_CSRF_TOKEN")),
		status:       "Waiting to launch Chrome...",
	}

	return client, nil
}

func (c *Client) Close() {
	if c.browserStop != nil {
		c.browserStop()
		c.browserStop = nil
	}
	if c.allocCancel != nil {
		c.allocCancel()
		c.allocCancel = nil
	}
	if c.profileDir != "" {
		_ = os.RemoveAll(c.profileDir)
		c.profileDir = ""
	}
}

func (c *Client) Status() string {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.status
}

func (c *Client) ListConversations(ctx context.Context, pageSize int) ([]Conversation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureReady(ctx); err != nil {
		return nil, err
	}

	if pageSize <= 0 {
		pageSize = 100
	}

	var (
		offset        int
		conversations []Conversation
	)

	for {
		var page conversationList
		url := fmt.Sprintf("%s/conversations?offset=%d&limit=%d", backendAPIURL, offset, pageSize)
		respBody, err := c.doBackendRequest(ctx, httpMethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(respBody, &page); err != nil {
			return nil, fmt.Errorf("decode conversations response: %w", err)
		}
		if len(page.Items) == 0 {
			break
		}

		conversations = append(conversations, page.Items...)
		offset += pageSize
	}

	return conversations, nil
}

func (c *Client) ArchiveConversation(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureReady(ctx); err != nil {
		return err
	}

	_, err := c.doBackendRequest(ctx, httpMethodPatch, backendAPIURL+"/conversation/"+id, map[string]any{"is_archived": true})
	return err
}

func (c *Client) DeleteConversation(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureReady(ctx); err != nil {
		return err
	}

	_, err := c.doBackendRequest(ctx, httpMethodPatch, backendAPIURL+"/conversation/"+id, map[string]any{"is_visible": false})
	return err
}

func (c *Client) ensureReady(ctx context.Context) error {
	if c.browserCtx == nil {
		c.setStatus("Launching Chrome window...")
		if err := c.startBrowser(); err != nil {
			c.Close()
			return err
		}
	}
	if c.accessToken != "" {
		return nil
	}
	c.setStatus("Waiting for a valid ChatGPT session in Chrome...")
	return c.waitForAccessToken(ctx)
}

func (c *Client) startBrowser() error {
	chromePath, err := resolveChromePath()
	if err != nil {
		return err
	}

	profileDir, err := os.MkdirTemp("", "chatgpt-bulk-chrome-*")
	if err != nil {
		return fmt.Errorf("create browser profile dir: %w", err)
	}
	c.profileDir = profileDir

	windowsProfileDir, err := wslToWindowsPath(profileDir)
	if err != nil {
		return fmt.Errorf("convert browser profile path: %w", err)
	}

	debugPort, err := pickFreePort()
	if err != nil {
		return fmt.Errorf("pick debug port: %w", err)
	}
	args := []string{
		"https://chatgpt.com/",
		fmt.Sprintf("--remote-debugging-port=%d", debugPort),
		"--remote-debugging-address=0.0.0.0",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-popup-blocking",
		"--start-maximized",
		fmt.Sprintf("--user-data-dir=%s", windowsProfileDir),
	}
	if parseBoolEnv("HEADLESS") {
		args = append([]string{"--headless=new"}, args...)
	}

	c.setStatus(fmt.Sprintf("Running Chrome debugger via '%s %s'", chromePath, args))
	time.Sleep(5 * time.Second)
	if err := launchDetachedChrome(chromePath, args); err != nil {
		return fmt.Errorf("launch chrome: %w", err)
	}

	c.setStatus(fmt.Sprintf("Waiting for Chrome debugger on port %d...", debugPort))
	wsURL, err := waitForDebuggerURL(context.Background(), debugPort, 60*time.Second)
	if err != nil {
		return fmt.Errorf("wait for chrome debugger: %w", err)
	}

	c.allocCtx, c.allocCancel = chromedp.NewRemoteAllocator(context.Background(), wsURL)
	c.browserCtx, c.browserStop = chromedp.NewContext(c.allocCtx)

	startCtx, cancel := context.WithTimeout(c.browserCtx, 90*time.Second)
	defer cancel()

	c.debugf("chrome debugger ready: %s", wsURL)
	c.debugf("enabling network domain")
	c.setStatus("Chrome debugger connected. Initializing browser session...")
	if err := chromedp.Run(startCtx, network.Enable()); err != nil {
		return fmt.Errorf("enable browser network: %w", err)
	}

	c.debugf("seeding chatgpt cookies")
	c.setStatus("Applying ChatGPT cookies in Chrome...")
	if err := c.seedCookies(startCtx, c.sessionToken, c.csrfToken); err != nil {
		return err
	}

	c.debugf("navigating to %s", chatGPTBaseURL)
	c.setStatus("Opening ChatGPT in Chrome...")
	if err := chromedp.Run(startCtx,
		chromedp.Navigate(chatGPTBaseURL),
		chromedp.Sleep(2*time.Second),
	); err != nil {
		return fmt.Errorf("load chatgpt.com: %w", err)
	}
	c.debugf("chatgpt page loaded")

	return nil
}

func (c *Client) waitForAccessToken(ctx context.Context) error {
	deadlineCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		c.debugf("requesting session json")
		c.setStatus("Checking ChatGPT session in Chrome...")
		var session sessionResponse
		resp, err := c.fetchJSON(deadlineCtx, httpMethodGet, sessionAPIURL, nil)
		if err == nil && resp.Status == http.StatusOK {
			if err := json.Unmarshal(resp.Body, &session); err != nil {
				return fmt.Errorf("decode session response: %w", err)
			}
			if session.AccessToken != "" {
				c.accessToken = session.AccessToken
				if err := c.initializeHTTPClient(); err != nil {
					return err
				}
				c.setStatus("Closing temporary Chrome window...")
				c.closeBrowserSession()
				c.setStatus("Fetching conversations from ChatGPT...")
				c.debugf("chatgpt session authenticated")
				return nil
			}
		}

		select {
		case <-deadlineCtx.Done():
			return errors.New("timed out waiting for a valid ChatGPT browser session; if Chrome opened a login page, finish logging in there and rerun the command")
		case <-ticker.C:
		}
	}
}

type debuggerVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func waitForDebuggerURL(ctx context.Context, port int, timeout time.Duration) (string, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		req, err := http.NewRequestWithContext(deadlineCtx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK {
				var version debuggerVersion
				if err := json.Unmarshal(body, &version); err == nil && version.WebSocketDebuggerURL != "" {
					return version.WebSocketDebuggerURL, nil
				}
			}
		}

		select {
		case <-deadlineCtx.Done():
			return "", deadlineCtx.Err()
		case <-ticker.C:
		}
	}
}

func pickFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("unexpected listener address type")
	}
	return addr.Port, nil
}

func (c *Client) seedCookies(ctx context.Context, sessionToken, csrfToken string) error {
	if err := network.SetCookie("__Secure-next-auth.session-token", sessionToken).
		WithDomain(".chatgpt.com").
		WithPath("/").
		WithSecure(true).
		WithHTTPOnly(true).
		Do(ctx); err != nil {
		return fmt.Errorf("set session cookie: %w", err)
	}

	if csrfToken != "" {
		if err := network.SetCookie("__Host-next-auth.csrf-token", csrfToken).
			WithPath("/").
			WithSecure(true).
			Do(ctx); err != nil {
			return fmt.Errorf("set csrf cookie: %w", err)
		}
	}

	return nil
}

func (c *Client) initializeHTTPClient() error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodGet, chatGPTBaseURL, nil)
	if err != nil {
		return err
	}

	jar.SetCookies(req.URL, []*http.Cookie{
		{
			Name:   "__Secure-next-auth.session-token",
			Value:  c.sessionToken,
			Domain: ".chatgpt.com",
			Path:   "/",
			Secure: true,
		},
	})
	if c.csrfToken != "" {
		jar.SetCookies(req.URL, []*http.Cookie{
			{
				Name:   "__Host-next-auth.csrf-token",
				Value:  c.csrfToken,
				Path:   "/",
				Secure: true,
			},
		})
	}

	c.httpClient = &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}
	return nil
}

func (c *Client) doBackendRequest(ctx context.Context, method, url string, payload any) ([]byte, error) {
	if c.httpClient == nil {
		return nil, errors.New("backend client is not initialized")
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = strings.NewReader(string(raw))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Origin", chatGPTBaseURL)
	req.Header.Set("Referer", chatGPTBaseURL+"/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	c.debugf("backend request method=%s url=%s status=%d body=%s", method, url, resp.StatusCode, truncate(respBody, 300))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, classifyAuthError(resp.StatusCode, respBody)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, truncate(respBody, 200))
	}
	return respBody, nil
}

func (c *Client) fetchJSON(ctx context.Context, method, url string, payload any) (browserResponse, error) {
	headers := map[string]string{
		"Accept": "application/json, text/plain, */*",
	}
	if c.accessToken != "" && url != sessionAPIURL {
		headers["Authorization"] = "Bearer " + c.accessToken
	}
	if payload != nil {
		headers["Content-Type"] = "application/json"
	}

	urlJS, _ := json.Marshal(url)
	methodJS, _ := json.Marshal(method)
	headersJS, _ := json.Marshal(headers)
	payloadJS := "null"
	if payload != nil {
		if raw, err := json.Marshal(payload); err == nil {
			payloadJS = string(raw)
		} else {
			return browserResponse{}, err
		}
	}

	script := fmt.Sprintf(`(async () => {
		const response = await fetch(%s, {
			method: %s,
			credentials: "include",
			headers: %s,
			body: %s === null ? undefined : JSON.stringify(%s),
		});
		const text = await response.text();
		let body = null;
		try {
			body = JSON.parse(text);
		} catch (_) {}
		return {status: response.status, text, body};
	})()`, string(urlJS), string(methodJS), string(headersJS), payloadJS, payloadJS)

	var resp browserResponse
	if err := c.evaluate(ctx, script, &resp); err != nil {
		return browserResponse{}, err
	}
	c.debugf("browser fetch method=%s url=%s status=%d body=%s", method, url, resp.Status, truncate([]byte(resp.Text), 300))
	return resp, nil
}

func (c *Client) evaluate(ctx context.Context, expression string, out any) error {
	return chromedp.Run(c.browserCtx, chromedp.ActionFunc(func(runCtx context.Context) error {
		evalCtx := runCtx
		if ctx != nil {
			evalCtx = ctx
		}

		result, exception, err := runtime.Evaluate(expression).
			WithAwaitPromise(true).
			WithReturnByValue(true).
			Do(evalCtx)
		if err != nil {
			return err
		}
		if exception != nil {
			return fmt.Errorf("javascript evaluation failed: %s", exception.Text)
		}
		if out == nil || result == nil {
			return nil
		}

		raw, err := json.Marshal(result.Value)
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, out)
	}))
}

func decodeResponse(resp browserResponse, out any) error {
	if resp.Status != 200 {
		if resp.Status == 401 || resp.Status == 403 {
			return classifyAuthError(resp.Status, []byte(resp.Text))
		}
		return fmt.Errorf("request failed with status %d: %s", resp.Status, truncate([]byte(resp.Text), 200))
	}
	if len(resp.Body) == 0 || string(resp.Body) == "null" {
		return errors.New("response body was empty")
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func ensureSuccess(resp browserResponse, id string) error {
	if resp.Status >= 200 && resp.Status < 300 {
		return nil
	}
	if resp.Status == 401 || resp.Status == 403 {
		return classifyAuthError(resp.Status, []byte(resp.Text))
	}
	return fmt.Errorf("update conversation %s failed with status %d: %s", id, resp.Status, truncate([]byte(resp.Text), 200))
}

func resolveChromePath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("CHROME_PATH")); envPath != "" {
		return envPath, nil
	}

	candidates := []string{
		"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files (x86)/Google/Chrome/Application/chrome.exe",
		"/mnt/c/Program Files/Microsoft/Edge/Application/msedge.exe",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", errors.New("no Chrome/Edge executable found; set CHROME_PATH to a browser executable")
}

func wslToWindowsPath(path string) (string, error) {
	out, err := exec.Command("wslpath", "-w", filepath.Clean(path)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func launchDetachedChrome(chromePath string, args []string) error {
	windowsChromePath, err := wslToWindowsPath(chromePath)
	if err != nil {
		return err
	}

	cmdArgs := []string{"/C", "start", "", windowsChromePath}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("cmd.exe", cmdArgs...)
	return cmd.Start()
}

func (c *Client) closeBrowserSession() {
	if c.browserCtx != nil {
		_ = chromedp.Run(c.browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
			return browser.Close().Do(ctx)
		}))
	}
	if c.browserStop != nil {
		c.browserStop()
		c.browserStop = nil
	}
	if c.allocCancel != nil {
		c.allocCancel()
		c.allocCancel = nil
	}
}

func parseBoolEnv(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	ok, err := strconv.ParseBool(v)
	return err == nil && ok
}

func (c *Client) debugf(format string, args ...any) {
	if c.debug {
		c.logger.Printf(format, args...)
	}
}

func (c *Client) setStatus(status string) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()
	c.status = status
}

func truncate(data []byte, limit int) string {
	if len(data) <= limit {
		return string(data)
	}
	return string(data[:limit]) + "..."
}

func classifyAuthError(statusCode int, body []byte) error {
	bodyText := strings.ToLower(string(body))

	switch statusCode {
	case 401:
		return &AuthError{
			StatusCode: statusCode,
			Message:    "authentication failed with status 401: the session token is missing, invalid, or expired",
		}
	case 403:
		message := "authentication failed with status 403: ChatGPT returned a bot protection or Cloudflare challenge"
		if strings.Contains(bodyText, "cf-browser-verification") || strings.Contains(bodyText, "cloudflare") {
			message += "; retry with a fresh browser session token or complete the challenge in the browser window"
		}
		return &AuthError{
			StatusCode: statusCode,
			Message:    message,
		}
	default:
		return &AuthError{
			StatusCode: statusCode,
			Message:    fmt.Sprintf("authentication failed with status %d", statusCode),
		}
	}
}

type chatTime struct {
	time.Time
}

func (t *chatTime) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		t.Time = time.Time{}
		return nil
	}

	var seconds float64
	if err := json.Unmarshal(data, &seconds); err == nil {
		whole := int64(seconds)
		nanos := int64((seconds - float64(whole)) * float64(time.Second))
		t.Time = time.Unix(whole, nanos).UTC()
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == "" {
		t.Time = time.Time{}
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, text)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

const (
	httpMethodGet   = "GET"
	httpMethodPatch = "PATCH"
)
