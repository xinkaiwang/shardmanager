package kcommon

import "sync"

func RunWithLock(m *sync.Mutex, fnc func()) {
	m.Lock()
	defer m.Unlock()
	fnc()
}
