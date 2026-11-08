package viewer

import (
	"container/list"
	"context"

	"github.com/PranavJoshi2893/codebound/internal/calltree"
)

const maxCachedViews = 8

type cachedView struct {
	options calltree.BrowserOptions
	svg     []byte
}

// A small byte-bounded LRU avoids re-running Graphviz when following a path back
// or toggling context detail. The slot bounds concurrent layouts and also guards
// the cache; it is released BEFORE writing to a potentially slow HTTP client.
type viewCache struct {
	slot   chan struct{}
	items  map[calltree.BrowserOptions]*list.Element
	recent list.List
	bytes  int
	limit  int
	render func(context.Context, calltree.BrowserOptions) ([]byte, error)
}

func newViewCache(render func(context.Context, calltree.BrowserOptions) ([]byte, error)) *viewCache {
	return &viewCache{slot: make(chan struct{}, 1), items: make(map[calltree.BrowserOptions]*list.Element), limit: maxSVGBytes, render: render}
}

func (cache *viewCache) image(ctx context.Context, options calltree.BrowserOptions) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, renderTimeout)
	defer cancel()
	select {
	case cache.slot <- struct{}{}:
		defer func() { <-cache.slot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if hit := cache.items[options]; hit != nil {
		cache.recent.MoveToFront(hit)
		return hit.Value.(cachedView).svg, nil
	}
	svg, err := cache.render(ctx, options)
	if err != nil {
		return nil, err
	}
	if len(svg) > cache.limit {
		return svg, nil
	}
	for cache.bytes+len(svg) > cache.limit || cache.recent.Len() >= maxCachedViews {
		oldest := cache.recent.Back()
		view := oldest.Value.(cachedView)
		delete(cache.items, view.options)
		cache.bytes -= len(view.svg)
		cache.recent.Remove(oldest)
	}
	cache.items[options] = cache.recent.PushFront(cachedView{options: options, svg: svg})
	cache.bytes += len(svg)
	return svg, nil
}
