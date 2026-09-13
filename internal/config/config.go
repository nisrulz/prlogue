package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/nisrulz/prlogue/internal/sysinfo"
	"github.com/spf13/viper"
)

const (
	configName        = "config"
	configType        = "yaml"
	projectConfigFile = ".prlogue.yaml"
	maxConfigBytes    = 1 << 20
	maxProjectBytes   = 64 << 10
	maxResponseTokens = 1 << 20
	configDirEnv      = "PRLOGUE_CONFIG_DIR"
)

const openAICompatProvider = "openai_compat"

const DefaultResponseMaxTokens = 8192

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
