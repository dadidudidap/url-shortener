package handlers

import (
	"errors"
	"testing"
)

func TestCreateUniqueCode(t *testing.T) {
	t.Run("retries until insert succeeds", func(t *testing.T) {
		calls := 0
		code, err := createUniqueCode(func(string) (int64, error) {
			calls++
			if calls < maxCodeAttempts {
				return 0, nil // code taken by a concurrent request
			}
			return 1, nil
		})
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if calls != maxCodeAttempts {
			t.Errorf("calls = %d, want %d", calls, maxCodeAttempts)
		}
		if len(code) != 6 {
			t.Errorf("code = %q, want 6 characters", code)
		}
	})

	t.Run("gives up when every attempt collides", func(t *testing.T) {
		calls := 0
		_, err := createUniqueCode(func(string) (int64, error) {
			calls++
			return 0, nil
		})
		if !errors.Is(err, errCodeExhausted) {
			t.Fatalf("err = %v, want errCodeExhausted", err)
		}
		if calls != maxCodeAttempts {
			t.Errorf("calls = %d, want %d", calls, maxCodeAttempts)
		}
	})

	t.Run("stops on insert error", func(t *testing.T) {
		boom := errors.New("db down")
		calls := 0
		_, err := createUniqueCode(func(string) (int64, error) {
			calls++
			return 0, boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
		if calls != 1 {
			t.Errorf("calls = %d, want 1", calls)
		}
	})
}
