package probe

import (
	"testing"
	"time"
)

func TestAfterTimeStore_LookupUnknown(t *testing.T) {
	s := NewAfterTimeStore()
	if _, ok := s.Lookup("missing"); ok {
		t.Fatal("expected unknown id to be absent")
	}
}

func TestAfterTimeStore_SetThenLookup(t *testing.T) {
	s := NewAfterTimeStore()
	want := time.Now()
	s.Set("id", want)

	got, ok := s.Lookup("id")
	if !ok {
		t.Fatal("expected id to be present after Set")
	}
	if !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
