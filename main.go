package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var version = "dev"

type TestEnv struct {
	ConfigDir   string
	APIBase     string
	AccessToken string
	Timeout     time.Duration
	Retries     int
	HTTPClient  *http.Client
}

type CLIError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Flag       string         `json:"flag,omitempty"`
	Got        any            `json:"got,omitempty"`
	ValidRange map[string]int `json:"valid_range,omitempty"`
	Example    string         `json:"example,omitempty"`
	RetryAfter string         `json:"retry_after,omitempty"`
}

type ErrorEnvelope struct {
	Error CLIError `json:"error"`
}

type CommandSpec struct {
	Usage          string         `json:"usage"`
	RequiresAuth   string         `json:"requires_auth,omitempty"`
	RequiredScopes []string       `json:"required_scopes,omitempty"`
	Flags          map[string]any `json:"flags,omitempty"`
	Destructive    bool           `json:"destructive,omitempty"`
}

type AgentContext struct {
	SchemaVersion     string                 `json:"schema_version"`
	CLI               map[string]string      `json:"cli"`
	API               map[string]any         `json:"api"`
	Conventions       map[string]any         `json:"conventions"`
	AvailableProfiles []string               `json:"available_profiles"`
	Commands          map[string]CommandSpec `json:"commands"`
	Delivery          map[string][]string    `json:"delivery"`
	Feedback          map[string]bool        `json:"feedback"`
}

type PageResponse struct {
	Records   []any  `json:"records"`
	NextToken string `json:"next_token"`
}

type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var osExit = os.Exit
var oauthCallbackTimeout = 3 * time.Minute
var execOpenCommand = defaultOpenCommand
var saveTokenFunc = saveToken
var randRead = rand.Read

func defaultOpenCommand(target string) *exec.Cmd { return exec.Command("open", target) }
func openBrowser(target string) error            { return execOpenCommand(target).Start() }

func main() {
	osExit(runMain(os.Args[1:]))
}

func runMain(args []string) int {
	stdout, stderr, code := Execute(args, TestEnv{})
	if stdout != "" {
		fmt.Fprint(os.Stdout, stdout)
		if !strings.HasSuffix(stdout, "\n") {
			fmt.Fprintln(os.Stdout)
		}
	}
	if stderr != "" {
		fmt.Fprint(os.Stderr, stderr)
		if !strings.HasSuffix(stderr, "\n") {
			fmt.Fprintln(os.Stderr)
		}
	}
	return code
}

func Execute(args []string, env TestEnv) (string, string, int) { return ExecuteWithEnv(args, env) }

func ExecuteWithEnv(args []string, env TestEnv) (string, string, int) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		return helpText(), "", 0
	}
	var stderr string
	var code int
	env, stderr, code = applyRuntimeFlags(args, env)
	if code != 0 {
		return "", stderr, code
	}
	switch args[0] {
	case "agent-context":
		return mustJSON(buildAgentContext(env)), "", 0
	case "version":
		return mustJSON(map[string]string{"version": version}), "", 0
	case "update":
		return handleUpdate(args[1:], env)
	case "workouts":
		return handleWorkouts(args[1:], env)
	case "sleep":
		return handleSleep(args[1:], env)
	case "cycles":
		return handleCycles(args[1:], env)
	case "recovery":
		return handleListLike("recovery", args[1:], env)
	case "mapping":
		return handleMapping(args[1:], env)
	case "user":
		return handleUser(args[1:], env)
	case "auth":
		return handleAuth(args[1:], env)
	case "feedback":
		return handleFeedback(args[1:], env)
	default:
		return "", errorJSON(CLIError{Code: "unknown_command", Message: fmt.Sprintf("unknown command %q", args[0]), Example: "whoop-cli agent-context"}), 2
	}
}

func handleListLike(resource string, args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 0 || pos[0] != "list" {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: fmt.Sprintf("%s requires subcommand list", resource), Example: fmt.Sprintf("whoop-cli %s list --json", resource)}), 2
	}
	endpoint, ok := listEndpoint(resource)
	if !ok {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: fmt.Sprintf("%s is not a list resource", resource)}), 2
	}
	return apiList(env, endpoint, args, fmt.Sprintf("whoop-cli %s list --limit 25 --json", resource))
}

func handleWorkouts(args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "workouts requires subcommand list or get", Example: "whoop-cli workouts list --json"}), 2
	}
	if pos[0] == "list" {
		return handleListLike("workouts", args, env)
	}
	if pos[0] == "get" && len(pos) >= 2 {
		return apiGET(env, "/v2/activity/workout/"+pathSegment(pos[1]))
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "workouts get requires <workout-id>", Example: "whoop-cli workouts get <workout-id> --json"}), 2
}

func handleSleep(args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "sleep requires subcommand list or get", Example: "whoop-cli sleep list --json"}), 2
	}
	if pos[0] == "list" {
		return handleListLike("sleep", args, env)
	}
	if pos[0] == "get" && len(pos) >= 2 {
		return apiGET(env, "/v2/activity/sleep/"+pathSegment(pos[1]))
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "sleep get requires <sleep-id>", Example: "whoop-cli sleep get <sleep-id> --json"}), 2
}

func handleCycles(args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "cycles requires subcommand list, get, sleep, or recovery", Example: "whoop-cli cycles list --json"}), 2
	}
	if pos[0] == "list" {
		return handleListLike("cycles", args, env)
	}
	if pos[0] == "get" && len(pos) >= 2 {
		return apiGET(env, "/v2/cycle/"+pathSegment(pos[1]))
	}
	if len(pos) >= 3 && pos[0] == "sleep" && pos[1] == "get" {
		return apiGET(env, "/v2/cycle/"+pathSegment(pos[2])+"/sleep")
	}
	if len(pos) >= 3 && pos[0] == "recovery" && pos[1] == "get" {
		return apiGET(env, "/v2/cycle/"+pathSegment(pos[2])+"/recovery")
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "invalid cycles invocation", Example: "whoop-cli cycles get <cycle-id> --json"}), 2
}

func handleMapping(args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 2 && pos[0] == "get" {
		return apiGET(env, "/v1/activity-mapping/"+pathSegment(pos[1]))
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "mapping get requires <activity-v1-id>", Example: "whoop-cli mapping get <activity-v1-id> --json"}), 2
}

func handleUser(args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 1 && pos[0] == "get" {
		return apiGET(env, "/v2/user/profile/basic")
	}
	if len(pos) == 2 && pos[0] == "body" && pos[1] == "get" {
		return apiGET(env, "/v2/user/measurement/body")
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "user requires get or body get", Example: "whoop-cli user get --json"}), 2
}

func handleAuth(args []string, env TestEnv) (string, string, int) {
	if len(args) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "auth requires subcommand", Example: "whoop-cli auth status --json"}), 2
	}
	if args[0] == "revoke" {
		if !hasFlag(args[1:], "--force") {
			return "", errorJSON(CLIError{Code: "force_required", Message: "auth revoke is destructive and requires --force", Example: "whoop-cli auth revoke --force --json"}), 2
		}
		return handleAuthRevoke(args[1:], env)
	}
	if args[0] == "status" {
		return handleAuthStatus(env), "", 0
	}
	if args[0] == "login" {
		return handleAuthLogin(args[1:], env)
	}
	if args[0] == "refresh" {
		return handleAuthRefresh(args[1:], env)
	}
	if args[0] == "logout" {
		return handleAuthLogout(env)
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "unknown auth subcommand", Example: "whoop-cli auth status --json"}), 2
}

type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	Scope        string `json:"scope,omitempty"`
	ObtainedAt   string `json:"obtained_at"`
}

func handleAuthStatus(env TestEnv) string {
	if env.AccessToken != "" {
		return mustJSON(map[string]any{"authenticated": true, "source": "test_env"})
	}
	if os.Getenv("WHOOP_ACCESS_TOKEN") != "" {
		return mustJSON(map[string]any{"authenticated": true, "source": "env"})
	}
	tok, err := loadSavedToken(env)
	if err != nil || tok.AccessToken == "" {
		return mustJSON(map[string]any{"authenticated": false, "hint": "Run whoop-cli auth login"})
	}
	out := map[string]any{"authenticated": true, "source": "saved_token", "scope": tok.Scope}
	if expiresAt := tokenExpiresAt(tok); expiresAt != "" {
		out["expires_at"] = expiresAt
	}
	return mustJSON(out)
}

func handleAuthLogout(env TestEnv) (string, string, int) {
	deleted, err := deleteSavedToken(env)
	if err != nil {
		return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
	}
	return mustJSON(map[string]any{"authenticated": false, "token_deleted": deleted}), "", 0
}

func handleAuthRevoke(args []string, env TestEnv) (string, string, int) {
	if hasFlag(args, "--dry-run") {
		return mustJSON(map[string]any{"revoked": false, "dry_run": true, "endpoint": "DELETE /v2/user/access"}), "", 0
	}
	_, status, stderr, code := apiRequestMethod(env, http.MethodDelete, "/v2/user/access", nil)
	if code != 0 {
		return "", stderr, code
	}
	deleted, err := deleteSavedToken(env)
	if err != nil {
		return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
	}
	return mustJSON(map[string]any{"revoked": true, "status": status, "token_deleted": deleted}), "", 0
}

func handleAuthLogin(args []string, env TestEnv) (string, string, int) {
	clientID := firstNonEmpty(flagValue(args, "--client-id"), os.Getenv("WHOOP_CLIENT_ID"))
	clientSecret := firstNonEmpty(flagValue(args, "--client-secret"), os.Getenv("WHOOP_CLIENT_SECRET"))
	redirectURI := firstNonEmpty(flagValue(args, "--redirect-uri"), "http://localhost:8787/callback")
	scopes := firstNonEmpty(flagValue(args, "--scopes"), "read:profile read:body_measurement read:cycles read:recovery read:sleep read:workout offline")
	state := firstNonEmpty(flagValue(args, "--state"), randomState())
	authURL := firstNonEmpty(flagValue(args, "--auth-url"), "https://api.prod.whoop.com/oauth/oauth2/auth")
	tokenURL := firstNonEmpty(flagValue(args, "--token-url"), "https://api.prod.whoop.com/oauth/oauth2/token")
	if clientID == "" {
		return "", errorJSON(CLIError{Code: "missing_oauth_credentials", Message: "auth login requires --client-id or WHOOP_CLIENT_ID", Example: "whoop-cli auth login --client-id <id> --print-url --json"}), 3
	}
	if len(state) != 8 {
		return "", errorJSON(CLIError{Code: "invalid_flag_value", Message: "--state must be exactly 8 characters", Flag: "--state", Got: state, Example: "whoop-cli auth login --state abcdefgh --json"}), 2
	}
	authorize := buildAuthorizeURL(authURL, clientID, redirectURI, scopes, state)
	if hasFlag(args, "--print-url") {
		return mustJSON(map[string]any{"authorization_url": authorize, "redirect_uri": redirectURI, "state": state}), "", 0
	}
	if clientSecret == "" {
		return "", errorJSON(CLIError{Code: "missing_oauth_credentials", Message: "auth login code exchange requires --client-secret or WHOOP_CLIENT_SECRET", Example: "whoop-cli auth login --client-id <id> --client-secret <secret> --code <code> --json"}), 3
	}
	code := flagValue(args, "--code")
	if code == "" {
		if !hasFlag(args, "--no-browser") {
			_ = openBrowser(authorize)
		}
		var err error
		code, err = waitForOAuthCode(redirectURI, state)
		if err != nil {
			return "", errorJSON(CLIError{Code: "oauth_callback_error", Message: err.Error(), Example: "Use --print-url, open it, then retry with --code <code> if callback capture fails"}), 1
		}
	}
	tok, stderr, exit := exchangeOAuthCodeWithEnv(env, tokenURL, clientID, clientSecret, redirectURI, code)
	if exit != 0 {
		return "", stderr, exit
	}
	path, err := saveTokenFunc(env, tok)
	if err != nil {
		return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
	}
	return mustJSON(map[string]any{"token_saved": true, "path": path, "expires_in": tok.ExpiresIn, "scope": tok.Scope}), "", 0
}

func buildAuthorizeURL(base, clientID, redirectURI, scopes, state string) string {
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", scopes)
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

func waitForOAuthCode(redirectURI, expectedState string) (string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return "", err
	}
	defer ln.Close()
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != expectedState {
			errCh <- fmt.Errorf("OAuth state mismatch")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("OAuth callback missing code")
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "WHOOP authorization complete. You can close this tab.")
		codeCh <- code
	})
	server.Handler = mux
	go func() { _ = server.Serve(ln) }()
	select {
	case code := <-codeCh:
		_ = server.Shutdown(context.Background())
		return code, nil
	case err := <-errCh:
		_ = server.Shutdown(context.Background())
		return "", err
	case <-time.After(oauthCallbackTimeout):
		_ = server.Shutdown(context.Background())
		return "", fmt.Errorf("timed out waiting for OAuth callback at %s", redirectURI)
	}
}

func exchangeOAuthCode(tokenURL, clientID, clientSecret, redirectURI, code string) (OAuthToken, string, int) {
	return exchangeOAuthCodeWithEnv(TestEnv{}, tokenURL, clientID, clientSecret, redirectURI, code)
}

func exchangeOAuthCodeWithEnv(env TestEnv, tokenURL, clientID, clientSecret, redirectURI, code string) (OAuthToken, string, int) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	return exchangeTokenFormWithEnv(env, tokenURL, form)
}

func handleAuthRefresh(args []string, env TestEnv) (string, string, int) {
	clientID := firstNonEmpty(flagValue(args, "--client-id"), os.Getenv("WHOOP_CLIENT_ID"))
	clientSecret := firstNonEmpty(flagValue(args, "--client-secret"), os.Getenv("WHOOP_CLIENT_SECRET"))
	tokenURL := firstNonEmpty(flagValue(args, "--token-url"), "https://api.prod.whoop.com/oauth/oauth2/token")
	if clientID == "" || clientSecret == "" {
		return "", errorJSON(CLIError{Code: "missing_oauth_credentials", Message: "auth refresh requires --client-id and --client-secret or WHOOP_CLIENT_ID/WHOOP_CLIENT_SECRET", Example: "whoop-cli auth refresh --client-id <id> --client-secret <secret> --json"}), 3
	}
	oldTok, err := loadSavedToken(env)
	if err != nil || oldTok.RefreshToken == "" {
		return "", errorJSON(CLIError{Code: "refresh_token_missing", Message: "WHOOP refresh token is missing; run whoop-cli auth login with offline scope", Example: "whoop-cli auth login --scopes \"read:profile read:body_measurement read:cycles read:recovery read:sleep read:workout offline\" --json"}), 3
	}
	newTok, stderr, exit := exchangeRefreshTokenWithEnv(env, tokenURL, clientID, clientSecret, oldTok.RefreshToken)
	if exit != 0 {
		return "", stderr, exit
	}
	path, err := saveTokenFunc(env, newTok)
	if err != nil {
		return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
	}
	return mustJSON(map[string]any{"token_refreshed": true, "path": path, "expires_in": newTok.ExpiresIn, "scope": newTok.Scope}), "", 0
}

func exchangeRefreshToken(tokenURL, clientID, clientSecret, refreshToken string) (OAuthToken, string, int) {
	return exchangeRefreshTokenWithEnv(TestEnv{}, tokenURL, clientID, clientSecret, refreshToken)
}

func exchangeRefreshTokenWithEnv(env TestEnv, tokenURL, clientID, clientSecret, refreshToken string) (OAuthToken, string, int) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "offline")
	return exchangeTokenFormWithEnv(env, tokenURL, form)
}

func exchangeTokenForm(tokenURL string, form url.Values) (OAuthToken, string, int) {
	return exchangeTokenFormWithEnv(TestEnv{}, tokenURL, form)
}

func exchangeTokenFormWithEnv(env TestEnv, tokenURL string, form url.Values) (OAuthToken, string, int) {
	resp, err := httpClient(env).PostForm(tokenURL, form)
	if err != nil {
		return OAuthToken{}, errorJSON(CLIError{Code: "network_error", Message: err.Error()}), 7
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OAuthToken{}, errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
	}
	if resp.StatusCode >= 400 {
		return OAuthToken{}, errorJSON(CLIError{Code: "oauth_token_error", Message: string(body), Got: resp.StatusCode}), 4
	}
	var tok OAuthToken
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return OAuthToken{}, errorJSON(CLIError{Code: "invalid_json", Message: "WHOOP token endpoint returned invalid token JSON"}), 5
	}
	tok.ObtainedAt = time.Now().UTC().Format(time.RFC3339)
	return tok, "", 0
}

func saveToken(env TestEnv, tok OAuthToken) (string, error) {
	dir := tokenConfigDir(env)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "token.json")
	b, _ := json.MarshalIndent(tok, "", "  ")
	return path, os.WriteFile(path, b, 0o600)
}

func loadSavedAccessToken(env TestEnv) string {
	tok, err := loadSavedToken(env)
	if err != nil {
		return ""
	}
	return tok.AccessToken
}

func loadSavedToken(env TestEnv) (OAuthToken, error) {
	path := filepath.Join(tokenConfigDir(env), "token.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return OAuthToken{}, err
	}
	var tok OAuthToken
	if err := json.Unmarshal(b, &tok); err != nil {
		return OAuthToken{}, err
	}
	return tok, nil
}

func deleteSavedToken(env TestEnv) (bool, error) {
	path := filepath.Join(tokenConfigDir(env), "token.json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func tokenConfigDir(env TestEnv) string {
	if env.ConfigDir != "" {
		return env.ConfigDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "whoop-cli")
}

func tokenExpiresAt(tok OAuthToken) string {
	if tok.ObtainedAt == "" || tok.ExpiresIn <= 0 {
		return ""
	}
	obtained, err := time.Parse(time.RFC3339, tok.ObtainedAt)
	if err != nil {
		return ""
	}
	return obtained.Add(time.Duration(tok.ExpiresIn) * time.Second).UTC().Format(time.RFC3339)
}

func randomState() string {
	b := make([]byte, 4)
	if _, err := randRead(b); err != nil {
		return "abcdefgh"
	}
	return hex.EncodeToString(b)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func handleFeedback(args []string, env TestEnv) (string, string, int) {
	if len(args) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "feedback requires create or list", Example: "whoop-cli feedback create \"message\" --json"}), 2
	}
	dir := env.ConfigDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share", "whoop-cli")
	}
	path := filepath.Join(dir, "feedback.jsonl")
	if args[0] == "create" {
		if len(args) < 2 || strings.HasPrefix(args[1], "--") {
			return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "feedback create requires text", Example: "whoop-cli feedback create \"error should enumerate values\" --json"}), 2
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
		}
		entry := map[string]any{"created_at": time.Now().UTC().Format(time.RFC3339), "text": args[1]}
		line := compactJSON(entry) + "\n"
		if err := appendFile(path, line); err != nil {
			return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
		}
		return mustJSON(map[string]any{"recorded": true, "path": path}), "", 0
	}
	if args[0] == "list" {
		b, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return mustJSON(map[string]any{"records": []any{}}), "", 0
		}
		if err != nil {
			return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
		}
		var records []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			if line == "" {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal([]byte(line), &rec); err != nil {
				return "", errorJSON(CLIError{Code: "invalid_json", Message: "feedback file contains invalid JSONL", Got: line}), 5
			}
			records = append(records, rec)
		}
		return mustJSON(map[string]any{"records": records}), "", 0
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "feedback requires create or list", Example: "whoop-cli feedback list --json"}), 2
}

func apiGET(env TestEnv, path string) (string, string, int) {
	body, status, stderr, code := apiRequest(env, path, nil)
	if code != 0 {
		return "", stderr, code
	}
	_ = status
	var anyJSON any
	if err := json.Unmarshal(body, &anyJSON); err != nil {
		return "", errorJSON(CLIError{Code: "invalid_json", Message: "WHOOP API returned non-JSON response"}), 5
	}
	return mustJSON(anyJSON), "", 0
}

func apiList(env TestEnv, path string, args []string, example string) (string, string, int) {
	limit, stderr, code := parseLimit(args, example)
	if code != 0 {
		return "", stderr, code
	}
	params := map[string]string{"limit": strconv.Itoa(limit)}
	if v := flagValue(args, "--start"); v != "" {
		params["start"] = v
	}
	if v := flagValue(args, "--end"); v != "" {
		params["end"] = v
	}
	if v := flagValue(args, "--cursor"); v != "" {
		params["nextToken"] = v
	}
	all := hasFlag(args, "--all")
	var records []any
	var next string
	for {
		body, _, reqErr, reqCode := apiRequest(env, path, params)
		if reqCode != 0 {
			return "", reqErr, reqCode
		}
		var page PageResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return "", errorJSON(CLIError{Code: "invalid_json", Message: "WHOOP API returned non-JSON page response"}), 5
		}
		records = append(records, page.Records...)
		next = page.NextToken
		if !all || next == "" {
			break
		}
		params["nextToken"] = next
	}
	truncated := next != "" && !all
	out := map[string]any{"records": records, "next_cursor": nil, "truncated": truncated, "limit": limit}
	if next != "" && !all {
		out["next_cursor"] = next
		out["hint"] = "Use --cursor " + next + " or --all to fetch additional pages."
	}
	return mustJSON(out), "", 0
}

func apiRequest(env TestEnv, path string, params map[string]string) ([]byte, int, string, int) {
	return apiRequestMethod(env, http.MethodGet, path, params)
}

func apiRequestMethod(env TestEnv, method, path string, params map[string]string) ([]byte, int, string, int) {
	base := env.APIBase
	if base == "" {
		base = os.Getenv("WHOOP_API_BASE")
	}
	if base == "" {
		base = "https://api.prod.whoop.com/developer"
	}
	token := env.AccessToken
	if token == "" {
		token = os.Getenv("WHOOP_ACCESS_TOKEN")
	}
	if token == "" {
		token = loadSavedAccessToken(env)
	}
	if token == "" {
		return nil, 0, errorJSON(CLIError{Code: "auth_missing", Message: "WHOOP access token is missing", Example: "Set WHOOP_ACCESS_TOKEN or run whoop-cli auth login --json"}), 3
	}
	reqURL := strings.TrimRight(base, "/") + path
	client := httpClient(env)
	retries := env.Retries
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequest(method, reqURL, nil)
		if err != nil {
			return nil, 0, errorJSON(CLIError{Code: "invalid_request", Message: err.Error()}), 2
		}
		q := req.URL.Query()
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "whoop-cli/"+version)
		resp, err := client.Do(req)
		if err != nil {
			if attempt < retries {
				continue
			}
			return nil, 0, errorJSON(CLIError{Code: "network_error", Message: err.Error()}), 7
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, resp.StatusCode, errorJSON(CLIError{Code: "io_error", Message: readErr.Error()}), 1
		}
		if shouldRetryStatus(resp.StatusCode) && attempt < retries {
			sleepBeforeRetry(resp.Header)
			continue
		}
		if resp.StatusCode >= 400 {
			return nil, resp.StatusCode, apiStatusError(resp.StatusCode, body, resp.Header), statusExitCode(resp.StatusCode)
		}
		return body, resp.StatusCode, "", 0
	}
}

func httpClient(env TestEnv) *http.Client {
	if env.HTTPClient != nil {
		return env.HTTPClient
	}
	timeout := env.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	client := *http.DefaultClient
	if client.Timeout == 0 {
		client.Timeout = timeout
	}
	return &client
}

func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

func statusExitCode(status int) int {
	if status == http.StatusUnauthorized {
		return 3
	}
	if status == http.StatusTooManyRequests {
		return 6
	}
	if status >= 500 {
		return 5
	}
	return 4
}

func apiStatusError(status int, body []byte, header http.Header) string {
	err := CLIError{Code: "whoop_api_error", Message: string(body), Got: status}
	if status == http.StatusTooManyRequests {
		err.RetryAfter = firstNonEmpty(header.Get("Retry-After"), header.Get("X-RateLimit-Reset"))
	}
	return errorJSON(err)
}

func sleepBeforeRetry(header http.Header) {
	delay := retryDelay(header)
	if delay > 0 {
		time.Sleep(delay)
	}
}

func retryDelay(header http.Header) time.Duration {
	raw := firstNonEmpty(header.Get("Retry-After"), header.Get("X-RateLimit-Reset"))
	if raw == "" {
		return 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 0
	}
	if seconds > 5 {
		seconds = 5
	}
	return time.Duration(seconds) * time.Second
}

func handleUpdate(args []string, env TestEnv) (string, string, int) {
	repo := firstNonEmpty(flagValue(args, "--repo"), "MohitTilwani15/whoop-cli")
	targetVersion := firstNonEmpty(flagValue(args, "--version"), "latest")
	releaseURL := flagValue(args, "--release-url")
	installDir := flagValue(args, "--install-dir")
	checkOnly := hasFlag(args, "--check")

	release, stderr, code := fetchRelease(env, repo, targetVersion, releaseURL)
	if code != 0 {
		return "", stderr, code
	}
	out := map[string]any{
		"current_version":  version,
		"latest_version":   release.TagName,
		"update_available": !sameVersion(version, release.TagName),
	}
	if checkOnly {
		return mustJSON(out), "", 0
	}
	if installDir == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
		}
		installDir = filepath.Dir(exe)
	}
	installedPath, err := installReleaseAsset(env, release, installDir)
	if err != nil {
		return "", errorJSON(CLIError{Code: "update_failed", Message: err.Error()}), 1
	}
	out["updated"] = true
	out["path"] = installedPath
	return mustJSON(out), "", 0
}

func fetchRelease(env TestEnv, repo, targetVersion, releaseURL string) (GitHubRelease, string, int) {
	if releaseURL == "" {
		if targetVersion == "latest" {
			releaseURL = "https://api.github.com/repos/" + repo + "/releases/latest"
		} else {
			releaseURL = "https://api.github.com/repos/" + repo + "/releases/tags/" + pathSegment(targetVersion)
		}
	}
	body, err := downloadBytes(env, releaseURL)
	if err != nil {
		return GitHubRelease{}, errorJSON(CLIError{Code: "network_error", Message: err.Error()}), 7
	}
	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil || release.TagName == "" {
		return GitHubRelease{}, errorJSON(CLIError{Code: "invalid_json", Message: "release endpoint returned invalid JSON"}), 5
	}
	return release, "", 0
}

func installReleaseAsset(env TestEnv, release GitHubRelease, installDir string) (string, error) {
	asset, ok := selectReleaseAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if !ok {
		return "", fmt.Errorf("no release asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	checksums, ok := selectChecksumsAsset(release.Assets)
	if !ok {
		return "", fmt.Errorf("release %s does not include checksums.txt", release.TagName)
	}
	archive, err := downloadBytes(env, asset.BrowserDownloadURL)
	if err != nil {
		return "", err
	}
	checksumBody, err := downloadBytes(env, checksums.BrowserDownloadURL)
	if err != nil {
		return "", err
	}
	if err := verifyAssetChecksum(asset.Name, archive, string(checksumBody)); err != nil {
		return "", err
	}
	binary, err := extractBinary(asset.Name, archive, updateBinaryName(runtime.GOOS))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(installDir, updateBinaryName(runtime.GOOS))
	if err := os.WriteFile(path, binary, 0o755); err != nil {
		return "", err
	}
	return path, nil
}

func downloadBytes(env TestEnv, target string) ([]byte, error) {
	if strings.HasPrefix(target, "file://") {
		return os.ReadFile(strings.TrimPrefix(target, "file://"))
	}
	resp, err := httpClient(env).Get(target)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s returned %d: %s", target, resp.StatusCode, string(body))
	}
	return body, nil
}

func selectReleaseAsset(assets []GitHubAsset, goos, goarch string) (GitHubAsset, bool) {
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if !strings.Contains(name, strings.ToLower(goos)) || !strings.Contains(name, strings.ToLower(goarch)) {
			continue
		}
		if strings.Contains(name, "checksum") || strings.Contains(name, "sha256") {
			continue
		}
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".zip") {
			return asset, true
		}
	}
	return GitHubAsset{}, false
}

func selectChecksumsAsset(assets []GitHubAsset) (GitHubAsset, bool) {
	for _, asset := range assets {
		name := strings.ToLower(asset.Name)
		if strings.HasSuffix(name, ".txt") && (strings.Contains(name, "checksum") || strings.Contains(name, "sha256")) {
			return asset, true
		}
	}
	return GitHubAsset{}, false
}

func verifyAssetChecksum(assetName string, body []byte, checksums string) error {
	expected := ""
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) == assetName {
			expected = strings.ToLower(fields[0])
			break
		}
	}
	if expected == "" {
		return fmt.Errorf("checksums.txt does not include %s", assetName)
	}
	sum := sha256.Sum256(body)
	actual := hex.EncodeToString(sum[:])
	if actual != expected {
		return fmt.Errorf("checksum mismatch for %s", assetName)
	}
	return nil
}

func extractBinary(assetName string, archive []byte, binaryName string) ([]byte, error) {
	lower := strings.ToLower(assetName)
	if strings.HasSuffix(lower, ".zip") {
		return extractBinaryZip(archive, binaryName)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractBinaryTarGz(archive, binaryName)
	}
	return nil, fmt.Errorf("unsupported archive format: %s", assetName)
}

func extractBinaryTarGz(archive []byte, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) != binaryName || h.FileInfo().IsDir() {
			continue
		}
		return io.ReadAll(tr)
	}
	return nil, fmt.Errorf("archive did not contain %s", binaryName)
}

func extractBinaryZip(archive []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != binaryName || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("archive did not contain %s", binaryName)
}

func updateBinaryName(goos string) string {
	if goos == "windows" {
		return "whoop-cli.exe"
	}
	return "whoop-cli"
}

func sameVersion(a, b string) bool {
	return strings.TrimPrefix(a, "v") == strings.TrimPrefix(b, "v")
}

func applyRuntimeFlags(args []string, env TestEnv) (TestEnv, string, int) {
	if value, found, ok := optionalFlagValue(args, "--api-base"); found {
		if !ok {
			return env, errorJSON(CLIError{Code: "missing_flag_value", Message: "--api-base requires a value", Flag: "--api-base", Example: "whoop-cli user get --api-base http://localhost:8080 --json"}), 2
		}
		env.APIBase = value
	}
	if value, found, ok := optionalFlagValue(args, "--config"); found {
		if !ok {
			return env, errorJSON(CLIError{Code: "missing_flag_value", Message: "--config requires a value", Flag: "--config", Example: "whoop-cli auth status --config ~/.config/whoop-cli --json"}), 2
		}
		env.ConfigDir = value
	}
	if value, found, ok := optionalFlagValue(args, "--timeout"); found {
		if !ok {
			return env, errorJSON(CLIError{Code: "missing_flag_value", Message: "--timeout requires a value", Flag: "--timeout", Example: "whoop-cli user get --timeout 30s --json"}), 2
		}
		timeout, err := parseTimeout(value)
		if err != nil {
			return env, errorJSON(CLIError{Code: "invalid_flag_value", Message: "--timeout must be a positive duration such as 30s or a positive number of seconds", Flag: "--timeout", Got: value, Example: "whoop-cli user get --timeout 30s --json"}), 2
		}
		env.Timeout = timeout
	}
	if value, found, ok := optionalFlagValue(args, "--retries"); found {
		if !ok {
			return env, errorJSON(CLIError{Code: "missing_flag_value", Message: "--retries requires a value", Flag: "--retries", Example: "whoop-cli user get --retries 2 --json"}), 2
		}
		retries, err := strconv.Atoi(value)
		if err != nil || retries < 0 || retries > 5 {
			return env, errorJSON(CLIError{Code: "invalid_flag_value", Message: "--retries must be between 0 and 5", Flag: "--retries", Got: value, ValidRange: map[string]int{"min": 0, "max": 5}, Example: "whoop-cli user get --retries 2 --json"}), 2
		}
		env.Retries = retries
	}
	return env, "", 0
}

func optionalFlagValue(args []string, flag string) (string, bool, bool) {
	for i := 0; i < len(args); i++ {
		if args[i] != flag {
			continue
		}
		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return "", true, false
		}
		return args[i+1], true, true
	}
	return "", false, false
}

func parseTimeout(value string) (time.Duration, error) {
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("timeout must be positive")
		}
		return time.Duration(seconds) * time.Second, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("timeout must be positive")
	}
	return d, nil
}

func pathSegment(value string) string {
	return url.PathEscape(value)
}

func listEndpoint(resource string) (string, bool) {
	switch resource {
	case "workouts":
		return "/v2/activity/workout", true
	case "sleep":
		return "/v2/activity/sleep", true
	case "cycles":
		return "/v2/cycle", true
	case "recovery":
		return "/v2/recovery", true
	default:
		return "", false
	}
}

func flagValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func parseLimit(args []string, example string) (int, string, int) {
	limit := 10
	for i := 0; i < len(args); i++ {
		if args[i] == "--limit" {
			if i+1 >= len(args) {
				return 0, errorJSON(CLIError{Code: "missing_flag_value", Message: "--limit requires a value", Flag: "--limit", Example: example}), 2
			}
			v, err := strconv.Atoi(args[i+1])
			if err != nil || v < 1 || v > 25 {
				got := any(args[i+1])
				if err == nil {
					got = v
				}
				return 0, errorJSON(CLIError{Code: "invalid_flag_value", Message: "--limit must be between 1 and 25", Flag: "--limit", Got: got, ValidRange: map[string]int{"min": 1, "max": 25}, Example: example}), 2
			}
			limit = v
			i++
		}
	}
	return limit, "", 0
}

func buildAgentContext(env TestEnv) AgentContext {
	commands := map[string]CommandSpec{
		"user.get":            {Usage: "whoop-cli user get --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:profile"}, Flags: dataFlags()},
		"user.body.get":       {Usage: "whoop-cli user body get --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:body_measurement"}, Flags: dataFlags()},
		"workouts.list":       {Usage: "whoop-cli workouts list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:workout"}, Flags: listFlags()},
		"workouts.get":        {Usage: "whoop-cli workouts get <workout-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:workout"}, Flags: dataFlags()},
		"sleep.list":          {Usage: "whoop-cli sleep list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:sleep"}, Flags: listFlags()},
		"sleep.get":           {Usage: "whoop-cli sleep get <sleep-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:sleep"}, Flags: dataFlags()},
		"cycles.list":         {Usage: "whoop-cli cycles list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:cycles"}, Flags: listFlags()},
		"cycles.get":          {Usage: "whoop-cli cycles get <cycle-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:cycles"}, Flags: dataFlags()},
		"cycles.sleep.get":    {Usage: "whoop-cli cycles sleep get <cycle-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:sleep"}, Flags: dataFlags()},
		"cycles.recovery.get": {Usage: "whoop-cli cycles recovery get <cycle-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:recovery"}, Flags: dataFlags()},
		"recovery.list":       {Usage: "whoop-cli recovery list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:recovery"}, Flags: listFlags()},
		"mapping.get":         {Usage: "whoop-cli mapping get <activity-v1-id> --json", RequiresAuth: "user_oauth", Flags: dataFlags()},
		"auth.login":          {Usage: "whoop-cli auth login --client-id <id> --client-secret <secret> --json", Flags: authLoginFlags()},
		"auth.status":         {Usage: "whoop-cli auth status --json", Flags: dataFlags()},
		"auth.refresh":        {Usage: "whoop-cli auth refresh --client-id <id> --client-secret <secret> --json", Flags: authRefreshFlags()},
		"auth.logout":         {Usage: "whoop-cli auth logout --json", Flags: dataFlags()},
		"auth.revoke":         {Usage: "whoop-cli auth revoke --force --json", RequiresAuth: "user_oauth", Destructive: true, Flags: destructiveFlags()},
		"feedback.create":     {Usage: "whoop-cli feedback create <text> --json", Flags: dataFlags()},
		"feedback.list":       {Usage: "whoop-cli feedback list --json", Flags: dataFlags()},
		"update":              {Usage: "whoop-cli update [--check] [--version <tag>] --json", Flags: updateFlags()},
	}
	return AgentContext{
		SchemaVersion:     "1",
		CLI:               map[string]string{"name": "whoop-cli", "version": version},
		API:               map[string]any{"base_url": "https://api.prod.whoop.com/developer", "auth": map[string]any{"oauth_authorization_url": "https://api.prod.whoop.com/oauth/oauth2/auth", "oauth_token_url": "https://api.prod.whoop.com/oauth/oauth2/token", "scopes": []string{"read:profile", "read:body_measurement", "read:cycles", "read:recovery", "read:sleep", "read:workout"}}},
		Conventions:       map[string]any{"json_flag": "--json", "destructive_bypass_flag": "--force", "non_interactive_flag": "--no-input", "pagination": map[string]string{"limit_flag": "--limit", "cursor_flag": "--cursor", "all_flag": "--all"}},
		AvailableProfiles: []string{"default"},
		Commands:          commands,
		Delivery:          map[string][]string{"schemes": {"stdout", "file:<path>", "webhook:<url>"}},
		Feedback:          map[string]bool{"local_enabled": true, "upstream_configured": os.Getenv("WHOOP_CLI_FEEDBACK_ENDPOINT") != ""},
	}
}

func dataFlags() map[string]any {
	return map[string]any{"--json": map[string]any{"type": "bool"}, "--profile": map[string]any{"type": "string"}, "--api-base": map[string]any{"type": "string"}, "--config": map[string]any{"type": "string"}, "--timeout": map[string]any{"type": "duration"}, "--retries": map[string]any{"type": "int", "min": 0, "max": 5, "default": 0}}
}
func listFlags() map[string]any {
	f := dataFlags()
	f["--limit"] = map[string]any{"type": "int", "min": 1, "max": 25, "default": 10}
	f["--start"] = map[string]any{"type": "string", "format": "date-time"}
	f["--end"] = map[string]any{"type": "string", "format": "date-time"}
	f["--cursor"] = map[string]any{"type": "string"}
	f["--all"] = map[string]any{"type": "bool"}
	return f
}
func destructiveFlags() map[string]any {
	f := dataFlags()
	f["--force"] = map[string]any{"type": "bool", "required": true}
	f["--dry-run"] = map[string]any{"type": "bool"}
	return f
}
func authLoginFlags() map[string]any {
	f := dataFlags()
	f["--client-id"] = map[string]any{"type": "string"}
	f["--client-secret"] = map[string]any{"type": "string", "secret": true}
	f["--redirect-uri"] = map[string]any{"type": "string"}
	f["--scopes"] = map[string]any{"type": "string"}
	f["--state"] = map[string]any{"type": "string", "length": 8}
	f["--auth-url"] = map[string]any{"type": "string"}
	f["--token-url"] = map[string]any{"type": "string"}
	f["--print-url"] = map[string]any{"type": "bool"}
	f["--code"] = map[string]any{"type": "string"}
	f["--no-browser"] = map[string]any{"type": "bool"}
	return f
}
func authRefreshFlags() map[string]any {
	f := dataFlags()
	f["--client-id"] = map[string]any{"type": "string"}
	f["--client-secret"] = map[string]any{"type": "string", "secret": true}
	f["--token-url"] = map[string]any{"type": "string"}
	return f
}
func updateFlags() map[string]any {
	f := dataFlags()
	f["--check"] = map[string]any{"type": "bool"}
	f["--version"] = map[string]any{"type": "string"}
	f["--repo"] = map[string]any{"type": "string", "default": "MohitTilwani15/whoop-cli"}
	f["--install-dir"] = map[string]any{"type": "string"}
	return f
}

func mustJSON(v any) string       { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }
func compactJSON(v any) string    { b, _ := json.Marshal(v); return string(b) }
func errorJSON(e CLIError) string { return mustJSON(ErrorEnvelope{Error: e}) }
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
func positionalArgs(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			if args[i] == "--limit" || args[i] == "--start" || args[i] == "--end" || args[i] == "--cursor" || args[i] == "--profile" || args[i] == "--api-base" || args[i] == "--config" || args[i] == "--timeout" || args[i] == "--retries" {
				i++
			}
			continue
		}
		out = append(out, args[i])
	}
	return out
}
func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(text)
	return err
}
func helpText() string {
	return "whoop-cli: agent-native CLI for WHOOP\n\nUse --json for data commands. Run `whoop-cli agent-context` for machine-readable command schema.\n"
}
