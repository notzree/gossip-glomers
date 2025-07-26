package main

import "sync"

type AtomicStorage interface {
	Read(key int) *int
	Write(key, value int)
}

type SimpleStorage struct {
	Mu    *sync.RWMutex
	Store map[int]int
}

func NewSimpleStorage() *SimpleStorage {
	return &SimpleStorage{
		Mu:    &sync.RWMutex{},
		Store: make(map[int]int),
	}
}

func (ss *SimpleStorage) Read(key int) *int {
	ss.Mu.RLock()
	defer ss.Mu.RUnlock()
	value, exist := ss.Store[key]
	if !exist {
		return nil
	}
	return &value
}
func (ss *SimpleStorage) Write(key, value int) {
	ss.Mu.Lock()
	defer ss.Mu.Unlock()
	ss.Store[key] = value
}
