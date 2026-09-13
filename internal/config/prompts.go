package config

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxOutputStyleLen = 1 << 16
	outputStyleFile   = "output_style_prompt.txt"
)

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
