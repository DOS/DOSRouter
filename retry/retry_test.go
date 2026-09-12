package retry

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type retryTestTransport func(*http.Request) (*http.Response, error)

func (f retryTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNetworkRetryOptOutDoesNotReplayAmbiguousPost(t *testing.T) {
	for _, disabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "disabled", false: "legacy-enabled"}[disabled], func(t *testing.T) {
			var attempts atomic.Int32
			client := &http.Client{Transport: retryTestTransport(func(r *http.Request) (*http.Response, error) {
				attempts.Add(1)
				_, _ = io.ReadAll(r.Body)
				return nil, errors.New("connection reset after upstream accepted request")
			})}
			_, err := Do(context.Background(), func() (*http.Request, error) {
				return http.NewRequest(http.MethodPost, "http://upstream.invalid/chat", strings.NewReader(`{"model":"paid"}`))
			}, WithClient(client), WithNetworkRetries(!disabled), WithBaseDelay(time.Microsecond))
			if err == nil {
				t.Fatal("expected transport failure")
			}
			want := int32(3)
			if disabled {
				want = 1
			}
			if got := attempts.Load(); got != want {
				t.Errorf("attempts = %d, want %d", got, want)
			}
		})
	}
}

func TestNetworkRetryOptOutRetainsRejectedStatusRetries(t *testing.T) {
	for _, status := range []int{429, 502, 503, 504} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var attempts atomic.Int32
			const payload = `{"model":"paid","messages":[]}`
			client := &http.Client{Transport: retryTestTransport(func(r *http.Request) (*http.Response, error) {
				got, err := io.ReadAll(r.Body)
				if err != nil || string(got) != payload {
					t.Errorf("retry body = %q, err = %v", got, err)
				}
				code := status
				if attempts.Add(1) == 2 {
					code = http.StatusOK
				}
				return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("response"))}, nil
			})}
			resp, err := Do(context.Background(), func() (*http.Request, error) {
				return http.NewRequest(http.MethodPost, "http://upstream.invalid/chat", strings.NewReader(payload))
			}, WithClient(client), WithNetworkRetries(false), WithBaseDelay(time.Microsecond))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK || attempts.Load() != 2 {
				t.Errorf("status/attempts = %d/%d", resp.StatusCode, attempts.Load())
			}
		})
	}
}

func TestCancellationStopsStatusBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var attempts atomic.Int32
	client := &http.Client{Transport: retryTestTransport(func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		cancel()
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"90"}}, Body: io.NopCloser(strings.NewReader("rejected"))}, nil
	})}
	start := time.Now()
	_, err := Do(ctx, func() (*http.Request, error) {
		return http.NewRequest(http.MethodPost, "http://upstream.invalid/chat", nil)
	}, WithClient(client), WithNetworkRetries(false))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if attempts.Load() != 1 {
		t.Errorf("cancelled request retried %d times", attempts.Load())
	}
	if time.Since(start) > time.Second {
		t.Error("cancellation waited for Retry-After")
	}
}
