package browser

import "io"

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
