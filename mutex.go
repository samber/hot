package hot

type rwMutex interface {
	Lock()
	Unlock()

	RLock()
	RUnlock()
}

type mutexMock struct {
}

var _ rwMutex = (*mutexMock)(nil)

func (f mutexMock) Lock()    {}
func (f mutexMock) Unlock()  {}
func (f mutexMock) RLock()   {}
func (f mutexMock) RUnlock() {}
