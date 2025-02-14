package idgen

import "sync"

type IdType interface {
	uint8 | uint16 | uint32 | uint64 | int8 | int16 | int32 | int64
}

type Generator[T IdType] interface {
	Allocate() T
	Free(T)
}

type generator[T IdType] struct {
	cur, lb, ub T
	values      map[T]bool //returned identities
	mutex       sync.Mutex
}

func NewGenerator[T IdType](lb, ub T) Generator[T] {
	return &generator[T]{
		lb:     lb,
		ub:     ub,
		values: make(map[T]bool),
		cur:    lb,
	}
}

func (g *generator[T]) Allocate() (id T) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	if l := len(g.values); l > 0 {
		for id, _ = range g.values {
			delete(g.values, id)
			break
		}
	} else {
		id = g.cur
		if g.cur++; g.cur >= g.ub {
			g.cur = g.lb
		}
	}
	return
}

func (g *generator[T]) Free(id T) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.values[id] = true
}
