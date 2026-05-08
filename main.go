package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

type TestEnv struct {
	ConfigDir   string
	APIBase     string
	AccessToken string
}

type CLIError struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Flag       string         `json:"flag,omitempty"`
	Got        any            `json:"got,omitempty"`
	ValidRange map[string]int `json:"valid_range,omitempty"`
	Example    string         `json:"example,omitempty"`
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

func main() {
	stdout, stderr, code := Execute(os.Args[1:], TestEnv{})
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
	os.Exit(code)
}

func Execute(args []string, env TestEnv) (string, string, int) { return ExecuteWithEnv(args, env) }

func ExecuteWithEnv(args []string, env TestEnv) (string, string, int) {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		return helpText(), "", 0
	}
	switch args[0] {
	case "agent-context":
		return mustJSON(buildAgentContext(env)), "", 0
	case "version":
		return mustJSON(map[string]string{"version": version}), "", 0
	case "workouts":
		return handleListLike("workouts", args[1:])
	case "sleep":
		return handleSleep(args[1:])
	case "cycles":
		return handleCycles(args[1:])
	case "recovery":
		return handleListLike("recovery", args[1:])
	case "user":
		return handleUser(args[1:], env)
	case "auth":
		return handleAuth(args[1:])
	case "feedback":
		return handleFeedback(args[1:], env)
	default:
		return "", errorJSON(CLIError{Code: "unknown_command", Message: fmt.Sprintf("unknown command %q", args[0]), Example: "whoop-pp-cli agent-context"}), 2
	}
}

func handleListLike(resource string, args []string) (string, string, int) {
	if len(args) == 0 || args[0] != "list" {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: fmt.Sprintf("%s requires subcommand list", resource), Example: fmt.Sprintf("whoop-pp-cli %s list --json", resource)}), 2
	}
	limit, stderr, code := parseLimit(args[1:], fmt.Sprintf("whoop-pp-cli %s list --limit 25 --json", resource))
	if code != 0 {
		return "", stderr, code
	}
	return mustJSON(map[string]any{"records": []any{}, "next_cursor": nil, "truncated": false, "limit": limit}), "", 0
}

func handleSleep(args []string) (string, string, int) {
	if len(args) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "sleep requires subcommand list or get", Example: "whoop-pp-cli sleep list --json"}), 2
	}
	if args[0] == "list" {
		return handleListLike("sleep", args)
	}
	if args[0] == "get" && len(args) >= 2 {
		return mustJSON(map[string]any{"id": args[1], "stub": true}), "", 0
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "sleep get requires <sleep-id>", Example: "whoop-pp-cli sleep get <sleep-id> --json"}), 2
}

func handleCycles(args []string) (string, string, int) {
	if len(args) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "cycles requires subcommand list, get, sleep, or recovery", Example: "whoop-pp-cli cycles list --json"}), 2
	}
	if args[0] == "list" {
		return handleListLike("cycles", args)
	}
	if args[0] == "get" && len(args) >= 2 {
		return mustJSON(map[string]any{"id": args[1], "stub": true}), "", 0
	}
	if len(args) >= 3 && (args[0] == "sleep" || args[0] == "recovery") && args[1] == "get" {
		return mustJSON(map[string]any{"cycle_id": args[2], "stub": true}), "", 0
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "invalid cycles invocation", Example: "whoop-pp-cli cycles get <cycle-id> --json"}), 2
}

func handleUser(args []string, env TestEnv) (string, string, int) {
	pos := positionalArgs(args)
	if len(pos) == 1 && pos[0] == "get" {
		return apiGET(env, "/v2/user/profile/basic")
	}
	if len(pos) == 2 && pos[0] == "body" && pos[1] == "get" {
		return apiGET(env, "/v2/user/measurement/body")
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "user requires get or body get", Example: "whoop-pp-cli user get --json"}), 2
}

func handleAuth(args []string) (string, string, int) {
	if len(args) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "auth requires subcommand", Example: "whoop-pp-cli auth status --json"}), 2
	}
	if args[0] == "revoke" {
		if !hasFlag(args[1:], "--force") {
			return "", errorJSON(CLIError{Code: "force_required", Message: "auth revoke is destructive and requires --force", Example: "whoop-pp-cli auth revoke --force --json"}), 2
		}
		return mustJSON(map[string]any{"revoked": true}), "", 0
	}
	if args[0] == "status" {
		return mustJSON(map[string]any{"authenticated": false, "hint": "Run whoop-pp-cli auth login"}), "", 0
	}
	if args[0] == "login" || args[0] == "refresh" || args[0] == "logout" {
		return mustJSON(map[string]any{"command": "auth." + args[0], "stub": true}), "", 0
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "unknown auth subcommand", Example: "whoop-pp-cli auth status --json"}), 2
}

func handleFeedback(args []string, env TestEnv) (string, string, int) {
	if len(args) == 0 {
		return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "feedback requires create or list", Example: "whoop-pp-cli feedback create \"message\" --json"}), 2
	}
	dir := env.ConfigDir
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share", "whoop-pp-cli")
	}
	path := filepath.Join(dir, "feedback.jsonl")
	if args[0] == "create" {
		if len(args) < 2 || strings.HasPrefix(args[1], "--") {
			return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "feedback create requires text", Example: "whoop-pp-cli feedback create \"error should enumerate values\" --json"}), 2
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
			_ = json.Unmarshal([]byte(line), &rec)
			records = append(records, rec)
		}
		return mustJSON(map[string]any{"records": records}), "", 0
	}
	return "", errorJSON(CLIError{Code: "invalid_invocation", Message: "feedback requires create or list", Example: "whoop-pp-cli feedback list --json"}), 2
}

func apiGET(env TestEnv, path string) (string, string, int) {
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
		return "", errorJSON(CLIError{Code: "auth_missing", Message: "WHOOP access token is missing", Example: "Set WHOOP_ACCESS_TOKEN or run whoop-pp-cli auth login --json"}), 3
	}
	url := strings.TrimRight(base, "/") + path
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", errorJSON(CLIError{Code: "invalid_request", Message: err.Error()}), 2
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "whoop-pp-cli/"+version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errorJSON(CLIError{Code: "network_error", Message: err.Error()}), 7
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errorJSON(CLIError{Code: "io_error", Message: err.Error()}), 1
	}
	if resp.StatusCode >= 400 {
		code := 4
		if resp.StatusCode == http.StatusUnauthorized {
			code = 3
		} else if resp.StatusCode == http.StatusTooManyRequests {
			code = 6
		} else if resp.StatusCode >= 500 {
			code = 5
		}
		return "", errorJSON(CLIError{Code: "whoop_api_error", Message: string(body), Got: resp.StatusCode}), code
	}
	var anyJSON any
	if err := json.Unmarshal(body, &anyJSON); err != nil {
		return "", errorJSON(CLIError{Code: "invalid_json", Message: "WHOOP API returned non-JSON response"}), 5
	}
	return mustJSON(anyJSON), "", 0
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
		"user.get":            {Usage: "whoop-pp-cli user get --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:profile"}, Flags: dataFlags()},
		"user.body.get":       {Usage: "whoop-pp-cli user body get --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:body_measurement"}, Flags: dataFlags()},
		"workouts.list":       {Usage: "whoop-pp-cli workouts list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:workout"}, Flags: listFlags()},
		"workouts.get":        {Usage: "whoop-pp-cli workouts get <workout-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:workout"}, Flags: dataFlags()},
		"sleep.list":          {Usage: "whoop-pp-cli sleep list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:sleep"}, Flags: listFlags()},
		"sleep.get":           {Usage: "whoop-pp-cli sleep get <sleep-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:sleep"}, Flags: dataFlags()},
		"cycles.list":         {Usage: "whoop-pp-cli cycles list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:cycles"}, Flags: listFlags()},
		"cycles.get":          {Usage: "whoop-pp-cli cycles get <cycle-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:cycles"}, Flags: dataFlags()},
		"cycles.sleep.get":    {Usage: "whoop-pp-cli cycles sleep get <cycle-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:sleep"}, Flags: dataFlags()},
		"cycles.recovery.get": {Usage: "whoop-pp-cli cycles recovery get <cycle-id> --json", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:recovery"}, Flags: dataFlags()},
		"recovery.list":       {Usage: "whoop-pp-cli recovery list [flags]", RequiresAuth: "user_oauth", RequiredScopes: []string{"read:recovery"}, Flags: listFlags()},
		"auth.revoke":         {Usage: "whoop-pp-cli auth revoke --force --json", RequiresAuth: "user_oauth", Destructive: true, Flags: map[string]any{"--json": map[string]any{"type": "bool"}, "--force": map[string]any{"type": "bool", "required": true}, "--dry-run": map[string]any{"type": "bool"}}},
		"feedback.create":     {Usage: "whoop-pp-cli feedback create <text> --json", Flags: dataFlags()},
	}
	return AgentContext{
		SchemaVersion:     "1",
		CLI:               map[string]string{"name": "whoop-pp-cli", "version": version},
		API:               map[string]any{"base_url": "https://api.prod.whoop.com/developer", "auth": map[string]any{"oauth_authorization_url": "https://api.prod.whoop.com/oauth/oauth2/auth", "oauth_token_url": "https://api.prod.whoop.com/oauth/oauth2/token", "scopes": []string{"read:profile", "read:body_measurement", "read:cycles", "read:recovery", "read:sleep", "read:workout"}}},
		Conventions:       map[string]any{"json_flag": "--json", "destructive_bypass_flag": "--force", "non_interactive_flag": "--no-input", "pagination": map[string]string{"limit_flag": "--limit", "cursor_flag": "--cursor", "all_flag": "--all"}},
		AvailableProfiles: []string{"default"},
		Commands:          commands,
		Delivery:          map[string][]string{"schemes": {"stdout", "file:<path>", "webhook:<url>"}},
		Feedback:          map[string]bool{"local_enabled": true, "upstream_configured": os.Getenv("WHOOP_CLI_FEEDBACK_ENDPOINT") != ""},
	}
}

func dataFlags() map[string]any {
	return map[string]any{"--json": map[string]any{"type": "bool"}, "--profile": map[string]any{"type": "string"}}
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
	return "whoop-pp-cli: agent-native CLI for WHOOP\n\nUse --json for data commands. Run `whoop-pp-cli agent-context` for machine-readable command schema.\n"
}
