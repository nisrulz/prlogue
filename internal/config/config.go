package config

import (
	"embed"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nisrulz/prlogue/internal/sysinfo"
	"github.com/spf13/viper"
)

const (
	configName        = "config"
	configType        = "yaml"
	projectConfigFile = ".prlogue.yaml"
	maxConfigBytes    = 1 << 20
	maxProjectBytes   = 64 << 10
	maxOutputStyleLen = 1 << 16
	maxResponseTokens = 1 << 20
	configDirEnv      = "PRLOGUE_CONFIG_DIR"
	outputStyleFile   = "output_style_prompt.txt"
)

const openAICompatProvider = "openai_compat"

const DefaultResponseMaxTokens = 8192

//go:embed prompts/*.txt
var promptFiles embed.FS

func DefaultPrompt() string {
	return readPromptFile("default_prompt.txt")
}

func DefaultOutputStylePrompt() string {
	return readPromptFile("output_style_prompt.txt")
}

func SecurityPrompt() string {
	return readPromptFile("security_prompt.txt")
}

func SanitizationPrompt() string {
	return readPromptFile("sanitization_prompt.txt")
}

func CommitSummaryPrompt() string {
	return readPromptFile("commit_summary_prompt.txt")
}

func readPromptFile(name string) string {
	data, err := promptFiles.ReadFile("prompts/" + name)
	if err != nil {
		panic(fmt.Sprintf("read embedded prompt %q: %v", name, err))
	}
	return string(data)
}

// OutputStylePromptPath returns the path users can edit to change formatting.
func OutputStylePromptPath() (string, error) {
	base, err := configBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "prlogue", outputStyleFile), nil
}

// EnsureOutputStylePromptFile creates the user style file when it is missing.
func EnsureOutputStylePromptFile() (string, error) {
	path, err := OutputStylePromptPath()
	if err != nil {
		return "", err
	}
	info, statErr := os.Lstat(path)
	if statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("output style prompt %s must be a regular file", path)
		}
		return path, nil
	}
	if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("stat output style prompt %s: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("mkdir output style prompt directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(DefaultOutputStylePrompt()), 0600); err != nil {
		return "", fmt.Errorf("write output style prompt: %w", err)
	}
	return path, nil
}

// LoadOutputStylePrompt loads the user style file or the embedded fallback.
func LoadOutputStylePrompt() string {
	path, err := OutputStylePromptPath()
	if err != nil {
		return DefaultOutputStylePrompt()
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, ensureErr := EnsureOutputStylePromptFile(); ensureErr != nil {
			return DefaultOutputStylePrompt()
		}
		return DefaultOutputStylePrompt()
	}
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > maxOutputStyleLen {
		return DefaultOutputStylePrompt()
	}
	data, err := os.ReadFile(path)
	if err != nil || !isOutputStylePrompt(string(data)) {
		return DefaultOutputStylePrompt()
	}
	return string(data)
}

func isOutputStylePrompt(prompt string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return false
	}
	for _, marker := range []string{
		"ignore previous",
		"ignore all instructions",
		"run ",
		"execute ",
		"shell",
		"git ",
		"secret",
		"credential",
		"password",
		"api key",
		"tool call",
		"network request",
		"file operation",
		"security policy",
	} {
		if strings.Contains(prompt, marker) {
			return false
		}
	}
	for _, marker := range []string{
		"format",
		"title",
		"heading",
		"section",
		"bullet",
		"markdown",
		"summary",
		"description",
		"concise",
		"pull request",
	} {
		if strings.Contains(prompt, marker) {
			return true
		}
	}
	return false
}

type Config struct {
	Name              string         `mapstructure:"name"`
	Provider          string         `mapstructure:"provider"`
	Model             string         `mapstructure:"model"`
	BaseURL           string         `mapstructure:"base_url"`
	ResponseMaxTokens int            `mapstructure:"response_max_tokens"`
	APIKey            string         `mapstructure:"-"`
	NoThink           bool           `mapstructure:"no_think"`
	StagedContext     bool           `mapstructure:"staged_context"`
	ExtraBody         map[string]any `mapstructure:"extra_body"`
	Context           ContextConf    `mapstructure:"context"`
	Chunking          ChunkConf      `mapstructure:"chunking"`
	Git               GitConf        `mapstructure:"git"`
	Output            OutputConf     `mapstructure:"output"`
	System            SystemConf     `mapstructure:"system"`
}

type ContextConf struct {
	Mode       string `mapstructure:"mode"` // auto | manual
	Manual     int    `mapstructure:"manual"`
	MaxAuto    int    `mapstructure:"max_auto"`
	MinAuto    int    `mapstructure:"min_auto"`
	calculated int
}

type ChunkConf struct {
	Strategy             string `mapstructure:"strategy"` // two-tier | file | hunk
	FileSummaryThreshold int    `mapstructure:"file_summary_threshold"`
	HunkSplitThreshold   int    `mapstructure:"hunk_split_threshold"`
}

type GitConf struct {
	DefaultBranch string `mapstructure:"default_branch"`
}

type OutputConf struct {
	Format string `mapstructure:"format"` // markdown | json
}

type SystemConf struct {
	OSReservationGB float64 `mapstructure:"os_reservation_gb"`
	ModelSizeGB     float64 `mapstructure:"model_size_gb"`
}

func (c *Config) ContextLength() int {
	if c.Context.Mode == "manual" {
		return c.Context.Manual
	}
	if c.Context.calculated == 0 {
		c.Context.calculated = calcAutoContext(c)
	}
	return c.Context.calculated
}

func (c *Config) ContextLengthWithRAM(ram *sysinfo.RAMInfo) int {
	if c.Context.Mode == "manual" {
		return c.Context.Manual
	}
	if c.Context.calculated == 0 {
		c.Context.calculated = calcAutoContextWithRAM(c, ram)
	}
	return c.Context.calculated
}

// DefaultConfig returns the initial configuration written to disk on first
// run. After that the config file is the source of truth. The loader fills in
// response_max_tokens for older files that do not contain the setting.
func DefaultConfig() *Config {
	return &Config{
		Name:              "Ollama",
		Provider:          openAICompatProvider,
		Model:             "lfm2.5:8b",
		BaseURL:           "http://localhost:11434/v1",
		ResponseMaxTokens: DefaultResponseMaxTokens,
		NoThink:           true,
		StagedContext:     true,
		Context: ContextConf{
			Mode:    "auto",
			Manual:  131072,
			MaxAuto: 1000000,
			MinAuto: 4096,
		},
		Chunking: ChunkConf{
			Strategy:             "two-tier",
			FileSummaryThreshold: 200,
			HunkSplitThreshold:   500,
		},
		Output: OutputConf{Format: "markdown"},
		System: SystemConf{ModelSizeGB: 5.2},
	}
}

// Load reads trusted user configuration. An explicit --config path is trusted.
// Without it, the user config is loaded from $PRLOGUE_CONFIG_DIR/prlogue/config.yaml
// (or ~/.config/prlogue/config.yaml when the env var is unset). If that file
// does not exist yet, it is created on first run with DefaultConfig. The
// current repository may override only git.default_branch and output.format.
func Load(path string) (*Config, error) {
	return load(path, true)
}

func LoadUser(path string) (*Config, error) {
	return load(path, false)
}

func load(path string, allowProjectOverrides bool) (*Config, error) {
	if path == "" {
		target, err := configPath("")
		if err != nil {
			return nil, err
		}
		if _, statErr := os.Stat(target); errors.Is(statErr, os.ErrNotExist) {
			if _, err := Save(DefaultConfig(), ""); err != nil {
				return nil, fmt.Errorf("create initial config: %w", err)
			}
		} else if statErr != nil {
			return nil, fmt.Errorf("stat config %s: %w", target, statErr)
		}
	}
	if path == "" {
		if _, err := EnsureOutputStylePromptFile(); err != nil {
			return nil, fmt.Errorf("create output style prompt: %w", err)
		}
	}

	v := newViper()
	if err := readTrustedConfig(v, path); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if !v.IsSet("response_max_tokens") {
		cfg.ResponseMaxTokens = DefaultResponseMaxTokens
	}
	if !v.IsSet("staged_context") {
		cfg.StagedContext = true
	}

	if path == "" && allowProjectOverrides {
		if err := applyProjectOverrides(&cfg); err != nil {
			return nil, err
		}
	}

	cfg.APIKey = os.Getenv("PRLOGUE_OPENAI_COMPAT_API_KEY")
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigType(configType)
	return v
}

func readTrustedConfig(v *viper.Viper, path string) error {
	target, err := configPath(path)
	if err != nil {
		return err
	}
	info, statErr := os.Stat(target)
	if statErr != nil {
		return fmt.Errorf("stat config %s: %w", target, statErr)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("config %s must be a regular file", target)
	}
	if info.Size() > maxConfigBytes {
		return fmt.Errorf("config %s exceeds %d bytes", target, maxConfigBytes)
	}

	v.SetConfigFile(target)
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("read config %s: %w", target, err)
	}
	return nil
}

// projectConfigRead validates and reads the repository's .prlogue.yaml.
// It returns (nil, nil) when no project config exists.
func projectConfigRead() (*viper.Viper, error) {
	info, err := os.Stat(projectConfigFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("stat project config: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("project config must be a regular file")
	}
	if info.Size() > maxProjectBytes {
		return nil, fmt.Errorf("project config exceeds %d bytes", maxProjectBytes)
	}

	v := viper.New()
	v.SetConfigFile(projectConfigFile)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read project config: %w", err)
	}

	allowed := map[string]bool{
		"git.default_branch": true,
		"output.format":      true,
	}
	for _, key := range v.AllKeys() {
		if !allowed[key] {
			return nil, fmt.Errorf("project config key %q is not allowed; move it to the user config or pass --config explicitly", key)
		}
	}
	return v, nil
}

// ProjectConfigErr reports a problem with the repository's .prlogue.yaml
// without applying its overrides. It returns nil when the file is absent or
// valid.
func ProjectConfigErr() error {
	_, err := projectConfigRead()
	return err
}

func applyProjectOverrides(cfg *Config) error {
	v, err := projectConfigRead()
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}

	if v.InConfig("git.default_branch") {
		cfg.Git.DefaultBranch = v.GetString("git.default_branch")
	}
	if v.InConfig("output.format") {
		cfg.Output.Format = v.GetString("output.format")
	}
	return nil
}

func (c *Config) Validate() error {
	if c.Provider != openAICompatProvider {
		return fmt.Errorf("unsupported provider: %s", c.Provider)
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("model must not be empty")
	}
	if err := ValidateBaseURL(c.BaseURL); err != nil {
		return err
	}
	if c.ResponseMaxTokens < DefaultResponseMaxTokens || c.ResponseMaxTokens > maxResponseTokens {
		return fmt.Errorf("response_max_tokens must be between %d and %d", DefaultResponseMaxTokens, maxResponseTokens)
	}
	if c.Context.Mode != "auto" && c.Context.Mode != "manual" {
		return fmt.Errorf("context.mode must be 'auto' or 'manual'")
	}
	if c.Context.Manual < 4096 || c.Context.MinAuto < 4096 || c.Context.MaxAuto < 4096 {
		return fmt.Errorf("context lengths must be at least 4096")
	}
	if c.Context.MinAuto > c.Context.MaxAuto {
		return fmt.Errorf("context.min_auto must not exceed context.max_auto")
	}
	switch c.Chunking.Strategy {
	case "two-tier", "file", "hunk":
	default:
		return fmt.Errorf("chunking.strategy must be 'two-tier', 'file', or 'hunk'")
	}
	if c.Chunking.FileSummaryThreshold <= 0 || c.Chunking.HunkSplitThreshold <= 0 {
		return fmt.Errorf("chunking thresholds must be greater than zero")
	}
	if c.Output.Format != "markdown" && c.Output.Format != "json" {
		return fmt.Errorf("output.format must be 'markdown' or 'json'")
	}
	if c.System.OSReservationGB < 0 || c.System.ModelSizeGB < 0 {
		return fmt.Errorf("system memory values must not be negative")
	}
	for key := range c.ExtraBody {
		switch key {
		case "model", "messages", "max_tokens", "temperature", "stream":
			return fmt.Errorf("extra_body must not override protected field %q", key)
		}
	}
	return nil
}

func ValidateBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid base_url %q", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("base_url must use http or https")
	}
	if u.User != nil {
		return fmt.Errorf("base_url must not contain credentials")
	}
	if u.Scheme == "http" && !IsLoopbackHost(u.Hostname()) {
		return fmt.Errorf("base_url must use https for non-loopback hosts")
	}
	return nil
}

// IsLoopbackHost reports whether host is localhost, a loopback address, or a
// private IP. Used to allow plain HTTP for local servers and to decide when a
// remote OpenAI-compatible endpoint needs an API key.
func IsLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func calcAutoContext(cfg *Config) int {
	ram, err := sysinfo.DetectRAM(cfg.System.OSReservationGB)
	if err != nil {
		return clampContext(cfg.Context.Manual, cfg.Context.MinAuto, cfg.Context.MaxAuto)
	}
	return calcAutoContextWithRAM(cfg, ram)
}

func calcAutoContextWithRAM(cfg *Config, ram *sysinfo.RAMInfo) int {
	if ram == nil {
		return clampContext(cfg.Context.Manual, cfg.Context.MinAuto, cfg.Context.MaxAuto)
	}
	maxCtx := sysinfo.CalcMaxContext(ram.AvailableRAMGB, cfg.Context.MaxAuto, cfg.System.ModelSizeGB)
	return clampContext(maxCtx, cfg.Context.MinAuto, cfg.Context.MaxAuto)
}

func clampContext(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func Save(cfg *Config, path string) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	target, err := configTarget(path)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("open config %s: %w", target, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close config %s: %w", target, err)
	}
	if err := os.Chmod(target, 0600); err != nil {
		return "", fmt.Errorf("secure config %s: %w", target, err)
	}

	v := viper.New()
	v.SetConfigType(configType)
	v.SetConfigFile(target)
	v.Set("name", cfg.Name)
	v.Set("provider", cfg.Provider)
	v.Set("model", cfg.Model)
	v.Set("base_url", cfg.BaseURL)
	v.Set("response_max_tokens", cfg.ResponseMaxTokens)
	v.Set("no_think", cfg.NoThink)
	v.Set("staged_context", cfg.StagedContext)
	v.Set("context.mode", cfg.Context.Mode)
	v.Set("context.manual", cfg.Context.Manual)
	v.Set("context.max_auto", cfg.Context.MaxAuto)
	v.Set("context.min_auto", cfg.Context.MinAuto)
	v.Set("chunking.strategy", cfg.Chunking.Strategy)
	v.Set("chunking.file_summary_threshold", cfg.Chunking.FileSummaryThreshold)
	v.Set("chunking.hunk_split_threshold", cfg.Chunking.HunkSplitThreshold)
	v.Set("git.default_branch", cfg.Git.DefaultBranch)
	v.Set("output.format", cfg.Output.Format)
	v.Set("system.os_reservation_gb", cfg.System.OSReservationGB)
	v.Set("system.model_size_gb", cfg.System.ModelSizeGB)
	if len(cfg.ExtraBody) > 0 {
		v.Set("extra_body", cfg.ExtraBody)
	}

	if err := v.WriteConfig(); err != nil {
		return "", fmt.Errorf("write config %s: %w", target, err)
	}
	return target, nil
}

// Reset backs up an existing config and writes a fresh default config.
// It returns the config path and an empty backup path when no config existed.
func Reset(path string) (target, backup string, err error) {
	target, err = configTarget(path)
	if err != nil {
		return "", "", err
	}

	info, statErr := os.Lstat(target)
	switch {
	case errors.Is(statErr, os.ErrNotExist):
	case statErr != nil:
		return "", "", fmt.Errorf("stat config %s: %w", target, statErr)
	case info.Mode()&os.ModeSymlink != 0:
		return "", "", fmt.Errorf("config %s must not be a symlink", target)
	case !info.Mode().IsRegular():
		return "", "", fmt.Errorf("config %s must be a regular file", target)
	default:
		backup, err = backupConfig(target)
		if err != nil {
			return "", "", err
		}
	}

	if _, err := Save(DefaultConfig(), target); err != nil {
		return "", backup, fmt.Errorf("write default config: %w", err)
	}
	return target, backup, nil
}

func backupConfig(target string) (string, error) {
	data, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("read config for backup: %w", err)
	}
	backup := fmt.Sprintf("%s.%s.bak", target, time.Now().UTC().Format("20060102-150405.000000000"))
	file, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("create config backup %s: %w", backup, err)
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write config backup %s: %w", backup, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close config backup %s: %w", backup, err)
	}
	return backup, nil
}

// UserConfigPath returns the resolved default user config path
// ($PRLOGUE_CONFIG_DIR/prlogue/config.yaml, or ~/.config/prlogue/config.yaml
// when the env var is unset) without creating any files.
func UserConfigPath() (string, error) {
	return configPath("")
}

func configTarget(path string) (string, error) {
	target, err := configPath(path)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(target)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	return target, nil
}

func configPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	base, err := configBaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "prlogue", configName+"."+configType), nil
}

func configBaseDir() (string, error) {
	base := os.Getenv(configDirEnv)
	if base != "" {
		return base, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}
