package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/nisrulz/prlogue/internal/sysinfo"
)

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
