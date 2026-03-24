package chatgpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

const (
	chatGPTBaseURL = "https://chatgpt.com"
	backendAPIURL  = chatGPTBaseURL + "/backend-api"
	sessionAPIURL  = chatGPTBaseURL + "/api/auth/session"
	sessionCookie  = "__Secure-next-auth.session-token"
	csrfCookie     = "__Host-next-auth.csrf-token"
	cookieChunkLen = 3800
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
	User        struct {
		Email string `json:"email"`
	} `json:"user"`
}

type Client struct {
	debug        bool
	headless     bool
	chromePath   string
	sessionToken string
	csrfToken    string
	accessToken  string
	userEmail    string
	authSource   AuthSource
	status       string
	httpClient   *http.Client
	debugPort    int
	allowLogin   bool
	saveAuth     func(AuthState) error

	allocCtx    context.Context
	allocCancel context.CancelFunc
	browserCtx  context.Context
	browserStop context.CancelFunc
	profileDir  string

	mu       sync.Mutex
	statusMu sync.RWMutex
	logsMu   sync.RWMutex
	logs     []string
}

type Config struct {
	SessionToken string
	CSRFToken    string
	AccessToken  string
	UserEmail    string
	AuthSource   AuthSource
	Debug        bool
	Headless     bool
	ChromePath   string
	AllowLogin   bool
	SaveAuth     func(AuthState) error
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

func New(config Config) (*Client, error) {
	client := &Client{
		debug:        config.Debug,
		sessionToken: strings.TrimSpace(config.SessionToken),
		csrfToken:    strings.TrimSpace(config.CSRFToken),
		accessToken:  strings.TrimSpace(config.AccessToken),
		userEmail:    strings.TrimSpace(config.UserEmail),
		authSource:   config.AuthSource,
		headless:     config.Headless,
		chromePath:   strings.TrimSpace(config.ChromePath),
		allowLogin:   config.AllowLogin,
		saveAuth:     config.SaveAuth,
		status:       "Ready",
	}
	if client.accessToken != "" {
		client.status = "Using saved ChatGPT session..."
	} else if client.sessionToken != "" {
		client.status = "Using saved ChatGPT cookies..."
	} else if client.allowLogin {
		client.status = "Waiting to launch Chrome for ChatGPT login..."
	} else {
		client.status = "No ChatGPT auth configured."
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

func (c *Client) UserEmail() string {
	c.statusMu.RLock()
	defer c.statusMu.RUnlock()
	return c.userEmail
}

func (c *Client) SessionIDLabel() string {
	return MaskToken(c.sessionToken)
}

func (c *Client) Logs() []string {
	c.logsMu.RLock()
	defer c.logsMu.RUnlock()
	out := make([]string, len(c.logs))
	copy(out, c.logs)
	return out
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
			if retryErr := c.retryAuth(ctx, err); retryErr != nil {
				return nil, retryErr
			}
			respBody, err = c.doBackendRequest(ctx, httpMethodGet, url, nil)
			if err != nil {
				return nil, err
			}
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
	if err != nil {
		if retryErr := c.retryAuth(ctx, err); retryErr != nil {
			return retryErr
		}
		_, err = c.doBackendRequest(ctx, httpMethodPatch, backendAPIURL+"/conversation/"+id, map[string]any{"is_archived": true})
	}
	return err
}

func (c *Client) DeleteConversation(ctx context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureReady(ctx); err != nil {
		return err
	}

	_, err := c.doBackendRequest(ctx, httpMethodPatch, backendAPIURL+"/conversation/"+id, map[string]any{"is_visible": false})
	if err != nil {
		if retryErr := c.retryAuth(ctx, err); retryErr != nil {
			return retryErr
		}
		_, err = c.doBackendRequest(ctx, httpMethodPatch, backendAPIURL+"/conversation/"+id, map[string]any{"is_visible": false})
	}
	return err
}

func (c *Client) ensureReady(ctx context.Context) error {
	if c.httpClient == nil && c.sessionToken != "" {
		if err := c.initializeHTTPClient(); err != nil {
			return err
		}
	}
	if c.accessToken != "" {
		return nil
	}
	if c.browserCtx == nil {
		if c.sessionToken == "" && c.accessToken == "" && !c.allowLogin {
			return errors.New("no stored ChatGPT auth found and CHATGPT_SESSION_TOKEN is not set; run `chatgpt-bulk login`")
		}
		c.setStatus("Launching Chrome window...")
		if err := c.startBrowser(); err != nil {
			c.Close()
			return fmt.Errorf("browser launch failed: %w", err)
		}
	}
	c.setStatus("Waiting for a valid ChatGPT session in Chrome...")
	return c.waitForAccessToken(ctx)
}

func (c *Client) Authenticate(ctx context.Context) (AuthState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ensureReady(ctx); err != nil {
		return AuthState{}, err
	}
	return c.currentAuthState(), nil
}

func (c *Client) startBrowser() error {
	chromePath, err := resolveChromePath(c.chromePath)
	if err != nil {
		return err
	}

	profileDir, err := os.MkdirTemp("", "chatgpt-bulk-chrome-*")
	if err != nil {
		return fmt.Errorf("create browser profile dir: %w", err)
	}
	c.profileDir = profileDir

	debugPort, err := pickFreePort()
	if err != nil {
		return fmt.Errorf("pick debug port: %w", err)
	}
	c.debugPort = debugPort
	profileArg, err := profileDirForBrowser(chromePath, profileDir)
	if err != nil {
		return fmt.Errorf("convert browser profile path: %w", err)
	}
	args := []string{
		"https://chatgpt.com/",
		fmt.Sprintf("--remote-debugging-port=%d", debugPort),
		"--remote-debugging-address=0.0.0.0",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-popup-blocking",
		"--start-maximized",
		fmt.Sprintf("--user-data-dir=%s", profileArg),
	}
	if c.headless {
		args = append([]string{"--headless=new"}, args...)
	}

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

	if c.sessionToken != "" {
		c.debugf("seeding chatgpt cookies")
		c.setStatus("Applying saved ChatGPT cookies in Chrome...")
		if err := c.seedCookies(startCtx, c.sessionToken, c.csrfToken); err != nil {
			return err
		}
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
		if err != nil {
			c.debugf("session poll error: %v", err)
			if errors.Is(err, context.Canceled) ||
				strings.Contains(err.Error(), "context canceled") ||
				strings.Contains(err.Error(), "No target with given id found") {
				if reattachErr := c.reattachToLivePage(); reattachErr != nil {
					c.debugf("reattach failed: %v", reattachErr)
				} else {
					c.debugf("reattached to live page target")
				}
			}
		} else {
			c.debugf("session poll status=%d body=%s", resp.Status, truncate([]byte(resp.Text), 250))
		}
		if err == nil && resp.Status == http.StatusOK {
			if err := json.Unmarshal(resp.Body, &session); err != nil {
				return fmt.Errorf("decode session response: %w", err)
			}
			if session.AccessToken != "" {
				state, err := c.captureAuthState(deadlineCtx, session)
				if err != nil {
					return err
				}
				c.applyAuthState(state)
				if err := c.initializeHTTPClient(); err != nil {
					return err
				}
				if c.saveAuth != nil {
					if err := c.saveAuth(c.currentAuthState()); err != nil {
						return fmt.Errorf("auth file write failed: %w", err)
					}
				}
				c.setStatus("Closing temporary Chrome window...")
				c.closeBrowserSession()
				c.setStatus("Fetching conversations from ChatGPT...")
				c.debugf("chatgpt session authenticated")
				return nil
			}
			c.debugf("session poll returned 200 without accessToken")
		}

		select {
		case <-deadlineCtx.Done():
			return errors.New("session wait timed out: finish signing in or completing any challenge in the browser window, then rerun the command")
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
	return chromedp.Run(ctx, chromedp.ActionFunc(func(runCtx context.Context) error {
		for _, cookie := range splitCookieChunks(sessionCookie, sessionToken) {
			if err := network.SetCookie(cookie.Name, cookie.Value).
				WithDomain(".chatgpt.com").
				WithPath("/").
				WithSecure(true).
				WithHTTPOnly(true).
				Do(runCtx); err != nil {
				return fmt.Errorf("set session cookie %s: %w", cookie.Name, err)
			}
		}

		if csrfToken != "" {
			if err := network.SetCookie(csrfCookie, csrfToken).
				WithDomain("chatgpt.com").
				WithPath("/").
				WithSecure(true).
				Do(runCtx); err != nil {
				return fmt.Errorf("set csrf cookie: %w", err)
			}
		}

		return nil
	}))
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

	jar.SetCookies(req.URL, splitCookieChunks(sessionCookie, c.sessionToken))
	if c.csrfToken != "" {
		jar.SetCookies(req.URL, []*http.Cookie{
			{
				Name:   csrfCookie,
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

func (c *Client) currentAuthState() AuthState {
	return AuthState{
		SessionToken: c.sessionToken,
		CSRFToken:    c.csrfToken,
		AccessToken:  c.accessToken,
		UserEmail:    c.userEmail,
		SavedAt:      time.Now().UTC(),
		Source:       string(c.authSource),
	}
}

func (c *Client) applyAuthState(state AuthState) {
	c.sessionToken = strings.TrimSpace(state.SessionToken)
	c.csrfToken = strings.TrimSpace(state.CSRFToken)
	c.accessToken = strings.TrimSpace(state.AccessToken)
	c.userEmail = strings.TrimSpace(state.UserEmail)
	if source := strings.TrimSpace(state.Source); source != "" {
		c.authSource = AuthSource(source)
	}
}

func (c *Client) captureAuthState(ctx context.Context, session sessionResponse) (AuthState, error) {
	cookieCtx, cancel := c.browserCommandContext(ctx)
	defer cancel()

	var cookies []*network.Cookie
	if err := chromedp.Run(cookieCtx, chromedp.ActionFunc(func(runCtx context.Context) error {
		var err error
		cookies, err = network.GetCookies().WithURLs([]string{chatGPTBaseURL}).Do(runCtx)
		return err
	})); err != nil {
		return AuthState{}, fmt.Errorf("read auth cookies from browser: %w", err)
	}

	state := AuthState{
		AccessToken: strings.TrimSpace(session.AccessToken),
		UserEmail:   strings.TrimSpace(session.User.Email),
		SavedAt:     time.Now().UTC(),
		Source:      "browser",
	}

	state.SessionToken = readCookieValue(cookies, sessionCookie)
	state.CSRFToken = readCookieValue(cookies, csrfCookie)

	if state.SessionToken == "" {
		return AuthState{}, errors.New("authenticated browser session did not expose a ChatGPT session token")
	}
	if state.AccessToken == "" {
		return AuthState{}, errors.New("authenticated browser session did not return an access token")
	}

	return state, nil
}

func (c *Client) browserCommandContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.browserCtx == nil {
		return ctx, func() {}
	}

	if deadline, ok := ctx.Deadline(); ok {
		return context.WithDeadline(c.browserCtx, deadline)
	}

	return context.WithCancel(c.browserCtx)
}

func readCookieValue(cookies []*network.Cookie, name string) string {
	chunks := make(map[int]string)
	exactValue := ""
	maxChunk := -1

	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}

		cookieName := strings.TrimSpace(cookie.Name)
		switch {
		case cookieName == name:
			exactValue = strings.TrimSpace(cookie.Value)
		case strings.HasPrefix(cookieName, name+"."):
			suffix := strings.TrimPrefix(cookieName, name+".")
			index, err := strconv.Atoi(suffix)
			if err != nil || index < 0 {
				continue
			}
			chunks[index] = strings.TrimSpace(cookie.Value)
			if index > maxChunk {
				maxChunk = index
			}
		}
	}

	if exactValue != "" {
		return exactValue
	}
	if len(chunks) == 0 {
		return ""
	}

	var builder strings.Builder
	for i := 0; i <= maxChunk; i++ {
		chunk, ok := chunks[i]
		if !ok {
			return ""
		}
		builder.WriteString(chunk)
	}
	return builder.String()
}

func splitCookieChunks(name, value string) []*http.Cookie {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) <= cookieChunkLen {
		return []*http.Cookie{{
			Name:   name,
			Value:  value,
			Domain: ".chatgpt.com",
			Path:   "/",
			Secure: true,
		}}
	}

	var cookies []*http.Cookie
	for i, start := 0, 0; start < len(value); i, start = i+1, start+cookieChunkLen {
		end := start + cookieChunkLen
		if end > len(value) {
			end = len(value)
		}
		cookies = append(cookies, &http.Cookie{
			Name:   fmt.Sprintf("%s.%d", name, i),
			Value:  value[start:end],
			Domain: ".chatgpt.com",
			Path:   "/",
			Secure: true,
		})
	}
	return cookies
}

func (c *Client) retryAuth(ctx context.Context, err error) error {
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		return err
	}
	if c.sessionToken == "" && !c.allowLogin {
		return err
	}

	c.debugf("auth rejected, retrying with browser bootstrap: %v", authErr)
	c.closeBrowserSession()
	c.accessToken = ""
	c.httpClient = nil
	c.setStatus("Stored ChatGPT auth was rejected. Opening browser to refresh session...")
	if refreshErr := c.ensureReady(ctx); refreshErr != nil {
		return refreshErr
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

func (c *Client) evaluate(ctx context.Context, expression string, out any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			c.debugf("evaluate panic recovered: %v", r)
			err = fmt.Errorf("chrome evaluation panic: %v", r)
		}
	}()

	return chromedp.Run(c.browserCtx, chromedp.ActionFunc(func(runCtx context.Context) error {
		result, exception, err := runtime.Evaluate(expression).
			WithAwaitPromise(true).
			WithReturnByValue(true).
			Do(runCtx)
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

func resolveChromePath(override string) (string, error) {
	if override = strings.TrimSpace(override); override != "" {
		if goruntime.GOOS == "darwin" {
			override = filepath.Clean(override)
			if appBundle, ok := macAppBundlePath(override); ok {
				if _, err := os.Stat(appBundle); err == nil {
					return appBundle, nil
				}
			}
		}
		return override, nil
	}

	// Try PATH-based lookup first.
	pathCandidates := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"chrome",
		"msedge",
	}
	for _, candidate := range pathCandidates {
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved, nil
		}
	}

	// Try platform-specific well-known locations.
	for _, path := range platformChromeCandidates() {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", errors.New("no Chrome/Edge executable found; pass --chrome-path or install Chrome/Edge in a standard location")
}

func macAppBundlePath(chromePath string) (string, bool) {
	clean := filepath.Clean(chromePath)
	lower := strings.ToLower(clean)
	if strings.HasSuffix(lower, ".app") {
		return clean, true
	}

	marker := strings.ToLower(".app" + string(filepath.Separator))
	idx := strings.Index(lower, marker)
	if idx == -1 {
		return "", false
	}

	return clean[:idx+len(".app")], true
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

func (c *Client) reattachToLivePage() error {
	if c.debugPort == 0 {
		return errors.New("debug port is not set")
	}

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/json/list", c.debugPort), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var infos []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&infos); err != nil {
		return err
	}

	var chosen target.ID
	for _, info := range infos {
		if info.Type != "page" {
			continue
		}
		if strings.HasPrefix(info.URL, "chrome-extension://") {
			continue
		}
		chosen = target.ID(info.ID)
		if strings.Contains(info.URL, "chatgpt.com") || strings.Contains(info.URL, "auth.openai.com") {
			break
		}
	}
	if chosen == "" {
		return errors.New("no live page target found")
	}

	c.browserCtx, c.browserStop = chromedp.NewContext(c.allocCtx, chromedp.WithTargetID(chosen))
	return nil
}

func (c *Client) debugf(format string, args ...any) {
	if c.debug {
		c.logsMu.Lock()
		defer c.logsMu.Unlock()

		line := clampLogLine(fmt.Sprintf(format, args...), 180)
		c.logs = append(c.logs, time.Now().Format("15:04:05")+" "+line)
		if len(c.logs) > 80 {
			c.logs = append([]string(nil), c.logs[len(c.logs)-80:]...)
		}
	}
}

func clampLogLine(line string, width int) string {
	runes := []rune(strings.ReplaceAll(line, "\n", " "))
	if len(runes) <= width {
		return string(runes)
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
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
