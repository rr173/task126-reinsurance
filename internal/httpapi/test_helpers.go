package httpapi

import (
	"encoding/json"
	"io"
	"strconv"
)

// jsonNewDecoder wraps json.NewDecoder to keep the test file's imports tidy.
func jsonNewDecoder(r io.Reader) *json.Decoder {
	return json.NewDecoder(r)
}

// intToStr formats an int64 for path interpolation in tests.
func intToStr(n int64) string { return strconv.FormatInt(n, 10) }
