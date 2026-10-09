package netclient

import (
	"errors"
	"io"
)

const maxResponseBodyBytes = 1 << 20

const maxRedirectDrainBytes = 4 << 10

func readResponseBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxResponseBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponseBodyBytes {
		return nil, errors.New("response body exceeds 1 MiB limit")
	}
	return data, nil
}

func closeRedirectBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxRedirectDrainBytes))
	_ = body.Close()
}
