package mem

import (
	"testing"
	"time"
)

func TestMemSetGetDel(t *testing.T) {
	repo := New()

	err := repo.Set("k", []byte("v"), 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, ok, err := repo.Get("k")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("Get() ok = false, want true")
	}
	if string(got) != "v" {
		t.Fatalf("Get() value = %q, want %q", got, "v")
	}

	err = repo.Del("k")
	if err != nil {
		t.Fatalf("Del() error = %v", err)
	}

	_, ok, err = repo.Get("k")
	if err != nil {
		t.Fatalf("Get() after Del error = %v", err)
	}
	if ok {
		t.Fatalf("Get() after Del ok = true, want false")
	}
}

func TestMemTTL(t *testing.T) {
	repo := New()

	err := repo.Set("k", []byte("v"), 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	_, ok, err := repo.Get("k")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Fatalf("Get() ok = true, want false (expired)")
	}
}

func TestMemList(t *testing.T) {
	repo := New()
	err := repo.Set("gw:a", []byte("1"), 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	err = repo.Set("gw:b", []byte("2"), 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	err = repo.Set("other", []byte("3"), 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	keys, err := repo.ListKeys("gw:")
	if err != nil {
		t.Fatalf("ListKeys() error = %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("ListKeys() len = %d, want 1", len(keys))
	}
	hasGwA := false
	for _, key := range keys {
		if key == "gw:a" {
			hasGwA = true
		}
	}
	if !hasGwA {
		t.Fatalf("ListKeys() unexpected keys: %v", keys)
	}
}
