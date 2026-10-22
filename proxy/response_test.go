package proxy

import (
	"strings"
	"testing"
)

func TestUpstreamBodyBound(t *testing.T) {
	for _, size := range []int{7, 8, 9, 20} {
		input := strings.NewReader(strings.Repeat("x", size))
		body, err := readResponseBody(input, 8)
		if size <= 8 {
			if err != nil || len(body) != size {
				t.Fatalf("valid body rejected: %d", size)
			}
		} else {
			if err == nil || len(body) != 0 {
				t.Fatalf("oversized response retained: size=%d read=%d", size, len(body))
			}
			if input.Size()-int64(input.Len()) > 9 {
				t.Fatal("reader consumed past the bound")
			}
		}
	}
}
