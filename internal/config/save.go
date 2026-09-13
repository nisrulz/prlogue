package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

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
