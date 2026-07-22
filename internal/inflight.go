package internal

import "sync"

type flight struct {
	done chan struct{}
	body []byte
	err  error
}

type inflightGroup struct {
	mu sync.Mutex
	m  map[string]*flight
}

func newInflightGroup() *inflightGroup {
	return &inflightGroup{m: make(map[string]*flight)}
}

func (g *inflightGroup) do(key string, fn func() ([]byte, error)) ([]byte, error) {
	g.mu.Lock()
	if f, ok := g.m[key]; ok {
		g.mu.Unlock()
		<-f.done
		return f.body, f.err
	}
	f := &flight{done: make(chan struct{})}
	g.m[key] = f
	g.mu.Unlock()

	body, err := fn()
	f.body = body
	f.err = err
	close(f.done)

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return body, err
}
