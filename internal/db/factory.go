package db

import (
	"errors"
	"fmt"
	"sync"
)

type AdapterFactory func(dsn string) (Adapter, error)

var (
	registry   = make(map[string]AdapterFactory)
	registryMu sync.RWMutex
)

var ErrUnsupportedDatabase = errors.New("unsupported database type")

func Register(name string, factory AdapterFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

func Open(name, dsn string) (Adapter, error) {
	registryMu.RLock()
	factory, ok := registry[name]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedDatabase, name)
	}
	return factory(dsn)
}
