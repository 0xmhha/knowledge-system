package semantic

import (
	"crypto/sha256"
	"encoding/json"
	"sync"
)

// Retain only small coordinate values, never manifest bytes or validation
// outcomes. Each read still reads and hashes the complete current file.
// Source, anchors, chunks and snapshot alignment are validated as before.
const coordinateCacheEntries = 16

type coordinateCacheKey struct {
	kind byte
	sum  [32]byte
}

type coordinateDecodeCache struct {
	mu        sync.Mutex
	values    map[coordinateCacheKey]any
	order     []coordinateCacheKey
	unmarshal func([]byte, any) error
}

var decodedCoordinates coordinateDecodeCache

func (c *coordinateDecodeCache) decode(buf []byte, out any) error {
	var kind byte
	switch target := out.(type) {
	case *graphCoordinates:
		if target == nil || *target != (graphCoordinates{}) {
			return json.Unmarshal(buf, out)
		}
		kind = 'g'
	case *vectorCoordinates:
		if target == nil || *target != (vectorCoordinates{}) {
			return json.Unmarshal(buf, out)
		}
		kind = 'v'
	default:
		return json.Unmarshal(buf, out)
	}
	key := coordinateCacheKey{kind: kind, sum: sha256.Sum256(buf)}
	c.mu.Lock()
	defer c.mu.Unlock()
	if value, ok := c.values[key]; ok {
		switch target := out.(type) {
		case *graphCoordinates:
			*target = value.(graphCoordinates)
		case *vectorCoordinates:
			*target = value.(vectorCoordinates)
		}
		return nil
	}
	unmarshal := c.unmarshal
	if unmarshal == nil {
		unmarshal = json.Unmarshal
	}
	if err := unmarshal(buf, out); err != nil {
		return err // malformed manifests never warm the cache
	}
	var value any
	switch target := out.(type) {
	case *graphCoordinates:
		value = *target
	case *vectorCoordinates:
		value = *target
	}
	if c.values == nil {
		c.values = make(map[coordinateCacheKey]any)
	}
	if len(c.order) == coordinateCacheEntries {
		delete(c.values, c.order[0])
		c.order = c.order[1:]
	}
	c.values[key] = value
	c.order = append(c.order, key)
	return nil
}
