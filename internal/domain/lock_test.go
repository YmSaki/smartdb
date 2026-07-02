package domain

import (
	"testing"
	"time"
)

func TestProjectLockReadReadConcurrent(t *testing.T) {
	r := NewProjectLockRegistry()

	rel1, ok1 := r.TryReadLock("p1")
	if !ok1 {
		t.Fatal("first read lock should succeed")
	}
	defer rel1()

	rel2, ok2 := r.TryReadLock("p1")
	if !ok2 {
		t.Fatal("second concurrent read lock on same project should succeed (shared)")
	}
	defer rel2()
}

func TestProjectLockWriteExcludesRead(t *testing.T) {
	r := NewProjectLockRegistry()

	relW, okW := r.TryWriteLock("p1")
	if !okW {
		t.Fatal("write lock should succeed")
	}
	defer relW()

	if _, ok := r.TryReadLock("p1"); ok {
		t.Error("read lock should be rejected while write lock is held")
	}
}

func TestProjectLockWriteExcludesWrite(t *testing.T) {
	r := NewProjectLockRegistry()

	relW, okW := r.TryWriteLock("p1")
	if !okW {
		t.Fatal("first write lock should succeed")
	}
	defer relW()

	if _, ok := r.TryWriteLock("p1"); ok {
		t.Error("second write lock should be rejected while first is held")
	}
}

func TestProjectLockReadExcludesWrite(t *testing.T) {
	r := NewProjectLockRegistry()

	relR, okR := r.TryReadLock("p1")
	if !okR {
		t.Fatal("read lock should succeed")
	}
	defer relR()

	if _, ok := r.TryWriteLock("p1"); ok {
		t.Error("write lock should be rejected while a read lock is held")
	}
}

func TestProjectLockReleaseAllowsNextAcquire(t *testing.T) {
	r := NewProjectLockRegistry()

	relW, okW := r.TryWriteLock("p1")
	if !okW {
		t.Fatal("write lock should succeed")
	}
	relW()

	if _, ok := r.TryReadLock("p1"); !ok {
		t.Error("read lock should succeed after write lock released")
	}
}

func TestProjectLockIndependentProjectsDoNotBlock(t *testing.T) {
	r := NewProjectLockRegistry()

	relW, okW := r.TryWriteLock("p1")
	if !okW {
		t.Fatal("write lock on p1 should succeed")
	}
	defer relW()

	relOther, okOther := r.TryWriteLock("p2")
	if !okOther {
		t.Error("write lock on a different project should not be blocked by p1's lock")
	}
	if relOther != nil {
		defer relOther()
	}
}

func TestProjectLockDoesNotDeadlockUnderConcurrentUse(t *testing.T) {
	r := NewProjectLockRegistry()
	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			if rel, ok := r.TryReadLock("p1"); ok {
				rel()
			}
			if rel, ok := r.TryWriteLock("p1"); ok {
				rel()
			}
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("lock registry appears to deadlock under concurrent use")
	}
}
