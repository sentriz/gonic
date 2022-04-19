package iout

import (
	"errors"
	"fmt"
	"io"
)

func CopyRange(w io.Writer, r io.Reader, start, length int64) error {
	if _, err := io.CopyN(io.Discard, r, start); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("discard %d: %w", start, err)
	}
	if length == 0 {
		if _, err := io.Copy(w, r); err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("direct copy: %w", err)
		}
		return nil
	}
	if _, err := io.CopyN(w, io.MultiReader(r, NewNullReader()), length); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("copy %d: %w", length, err)
	}
	return nil
}
