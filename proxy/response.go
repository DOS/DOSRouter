package proxy

import (
	"errors"
	"io"
)

const (
	maxChatResponseBytes  int64 = 64 << 20
	maxErrorResponseBytes int64 = 1 << 20
)

func readResponseBody(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("upstream response too large")
	}
	return data, nil
}
