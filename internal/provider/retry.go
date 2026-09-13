package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

// HTTPError reports a non-2xx response from the raw extra_body HTTP path. The
// status code is preserved so callers can tell a retryable server failure from
// a permanent request error.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Body)
}

// retryPolicy bounds how often and how long Chat retries transient failures.
type retryPolicy struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

func defaultRetryPolicy() retryPolicy {
	return retryPolicy{
		maxAttempts: 4,
		baseDelay:   500 * time.Millisecond,
		maxDelay:    8 * time.Second,
	}
}

// isRetryable reports whether a failed request should be attempted again. Rate
// limits, server errors, and transport timeouts are transient. A canceled
// caller context is never retried.
func isRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return retryableStatus(httpErr.StatusCode)
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		return retryableStatus(apiErr.HTTPStatusCode)
	}
	var reqErr *openai.RequestError
	if errors.As(err, &reqErr) {
		return retryableStatus(reqErr.HTTPStatusCode)
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout, http.StatusConflict, http.StatusTooEarly, http.StatusTooManyRequests:
		return true
	}
	return code >= 500
}

// waitBackoff sleeps for an exponentially growing delay before the next
// attempt. It returns the context error when the caller cancels during the
// wait. Jitter spreads retries from parallel runs.
func waitBackoff(ctx context.Context, policy retryPolicy, attempt int) error {
	delay := policy.baseDelay
	for i := 1; i < attempt; i++ {
		if delay >= policy.maxDelay {
			break
		}
		delay *= 2
	}
	if delay > policy.maxDelay {
		delay = policy.maxDelay
	}
	delay += time.Duration(rand.Int64N(int64(delay/2) + 1))
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
