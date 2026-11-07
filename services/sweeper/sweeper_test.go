package main

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	batches [][]string
	calls   int
	errAt   int
	err     error
}

func (f *fakeStore) DeleteExpiredGuestURLs(_ context.Context, _ int32) ([]string, error) {
	call := f.calls
	f.calls++
	if f.err != nil && f.errAt == call {
		return nil, f.err
	}
	if call >= len(f.batches) {
		return nil, nil
	}
	return f.batches[call], nil
}

func TestSweepOnceDrainsBatches(t *testing.T) {
	store := &fakeStore{batches: [][]string{
		make([]string, 100),
		make([]string, 100),
		make([]string, 50),
	}}

	got, err := sweepOnce(context.Background(), store, 100)
	if err != nil {
		t.Fatalf("sweepOnce() error = %v", err)
	}
	if got != 250 {
		t.Errorf("sweepOnce() = %d, want 250", got)
	}
	if store.calls != 3 {
		t.Errorf("calls = %d, want 3", store.calls)
	}
}

func TestSweepOnceStopsOnError(t *testing.T) {
	wantErr := errors.New("database down")
	store := &fakeStore{
		batches: [][]string{make([]string, 100)},
		errAt:   1,
		err:     wantErr,
	}

	got, err := sweepOnce(context.Background(), store, 100)
	if !errors.Is(err, wantErr) {
		t.Fatalf("sweepOnce() error = %v, want %v", err, wantErr)
	}
	if got != 100 {
		t.Errorf("sweepOnce() = %d, want 100", got)
	}
	if store.calls != 2 {
		t.Errorf("calls = %d, want 2", store.calls)
	}
}
