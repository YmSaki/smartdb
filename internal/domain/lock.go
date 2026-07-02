package domain

import "sync"

// ProjectLockRegistry provides per-project, in-process mutual exclusion
// between normal SQL execution/backup (shared) and restore/migration
// (exclusive). See docs/spec.md §11 "Project Lock".
//
// Backup (VACUUM INTO) takes its own consistent snapshot under WAL, so it
// is treated as a shared/read operation that can run alongside normal SQL.
// Restore and migration replace/alter the database file itself, so they
// take the exclusive/write lock and block everything else for that
// project — including in-flight SQL — to avoid a request seeing a mix of
// pre-/post-restore data.
type ProjectLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*sync.RWMutex
}

func NewProjectLockRegistry() *ProjectLockRegistry {
	return &ProjectLockRegistry{locks: make(map[string]*sync.RWMutex)}
}

func (r *ProjectLockRegistry) get(projectID string) *sync.RWMutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.locks[projectID]
	if !ok {
		l = &sync.RWMutex{}
		r.locks[projectID] = l
	}
	return l
}

// TryReadLock attempts to acquire the shared lock for a project (normal
// SQL execution, backup). Returns a release function and true on success,
// or (nil, false) if a write lock (restore/migration) is currently held.
func (r *ProjectLockRegistry) TryReadLock(projectID string) (release func(), ok bool) {
	l := r.get(projectID)
	if !l.TryRLock() {
		return nil, false
	}
	return l.RUnlock, true
}

// TryWriteLock attempts to acquire the exclusive lock for a project
// (restore, migration). Returns a release function and true on success,
// or (nil, false) if any read or write lock is currently held.
func (r *ProjectLockRegistry) TryWriteLock(projectID string) (release func(), ok bool) {
	l := r.get(projectID)
	if !l.TryLock() {
		return nil, false
	}
	return l.Unlock, true
}
