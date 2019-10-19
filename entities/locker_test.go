package entities

import (
	"sync"
	"testing"
)

type mockLock struct {
	didLockFn   func()
	didUnlockFn func()
	actualLock  *sync.Mutex
}

func makeMockLock(lockCallback, unlockCallback func()) mockLock {
	return mockLock{
		didLockFn:   lockCallback,
		didUnlockFn: unlockCallback,
		actualLock:  new(sync.Mutex),
	}
}

func (l mockLock) Lock() {
	l.actualLock.Lock()
	l.didLockFn()
}

func (l mockLock) Unlock() {
	l.actualLock.Unlock()
	l.didUnlockFn()
}

func TestMultiLock_Lock(t *testing.T) {
	lockedAmount := 0
	lockFn := func() {
		lockedAmount++
	}
	unlockFn := func() {}

	mockLocks := []sync.Locker{
		makeMockLock(lockFn, unlockFn),
		makeMockLock(lockFn, unlockFn),
		makeMockLock(lockFn, unlockFn),
	}

	m := multiLock{mockLocks}
	m.Lock()

	if len(mockLocks) != lockedAmount {
		t.Errorf(
			"wanted %d locked mocks, only got %d",
			len(mockLocks),
			lockedAmount,
		)
	}
}
