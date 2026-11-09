package viewer

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PranavJoshi2893/codebound/internal/calltree/diagram"
)

func TestViewCache(t *testing.T) {
	renders := 0
	cache := newViewCache(func(context.Context, diagram.BrowserOptions) ([]byte, error) {
		renders++
		return []byte("1234"), nil
	})
	cache.limit = 8
	a, b, c := diagram.BrowserOptions{Focus: 0}, diagram.BrowserOptions{Focus: 1}, diagram.BrowserOptions{Focus: 2}
	for _, options := range []diagram.BrowserOptions{a, b, a, c, a} {
		if _, err := cache.image(context.Background(), options); err != nil {
			t.Fatal(err)
		}
	}
	if renders != 3 || cache.bytes != 8 || cache.items[b] != nil || cache.recent.Len() != 2 {
		t.Fatalf("cache must evict the least recently used view within its byte budget: renders=%d bytes=%d entries=%d", renders, cache.bytes, cache.recent.Len())
	}
	tooSmall := newViewCache(cache.render)
	tooSmall.limit = 3
	if _, err := tooSmall.image(context.Background(), b); err != nil {
		t.Fatal(err)
	}
	if tooSmall.recent.Len() != 0 || tooSmall.bytes != 0 {
		t.Fatal("oversized views must not be retained")
	}
}

func TestViewCacheConcurrentRequestsAndCancellation(t *testing.T) {
	var renders atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	cache := newViewCache(func(context.Context, diagram.BrowserOptions) ([]byte, error) {
		renders.Add(1)
		close(started)
		<-release
		return []byte("svg"), nil
	})
	options := diagram.BrowserOptions{Focus: 1}
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if _, err := cache.image(context.Background(), options); err != nil {
				t.Error(err)
			}
		})
	}
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cache.image(ctx, options); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled requests must not wait for another layout: %v", err)
	}
	close(release)
	group.Wait()
	if renders.Load() != 1 {
		t.Fatal("identical concurrent requests must share one layout")
	}
}

func TestViewCacheDoesNotRetainFailures(t *testing.T) {
	renders := 0
	cache := newViewCache(func(context.Context, diagram.BrowserOptions) ([]byte, error) {
		renders++
		return nil, errors.New("renderer failed")
	})
	for range 2 {
		if _, err := cache.image(context.Background(), diagram.BrowserOptions{}); err == nil {
			t.Fatal("renderer failures must be returned")
		}
	}
	if renders != 2 || cache.recent.Len() != 0 {
		t.Fatal("failed renders must remain retryable")
	}
}

func TestViewCacheEntryLimit(t *testing.T) {
	cache := newViewCache(func(context.Context, diagram.BrowserOptions) ([]byte, error) {
		return []byte("svg"), nil
	})
	for id := range maxCachedViews + 3 {
		if _, err := cache.image(context.Background(), diagram.BrowserOptions{Focus: id}); err != nil {
			t.Fatal(err)
		}
	}
	if cache.recent.Len() != maxCachedViews || cache.bytes != maxCachedViews*3 {
		t.Fatal("small views must still obey the entry-count bound")
	}
}

func TestRegistrationVisibilityHasItsOwnCacheKey(t *testing.T) {
	renders := 0
	cache := newViewCache(func(_ context.Context, options diagram.BrowserOptions) ([]byte, error) {
		renders++
		if options.HideRegistrations {
			return []byte("calls only"), nil
		}
		return []byte("calls and registrations"), nil
	})
	for _, hide := range []bool{false, true, false, true} {
		image, err := cache.image(context.Background(), diagram.BrowserOptions{Focus: 0, HideRegistrations: hide})
		if err != nil {
			t.Fatal(err)
		}
		if (string(image) == "calls only") != hide {
			t.Fatal("registration visibility reused the wrong cached diagram")
		}
	}
	if renders != 2 {
		t.Fatal("identical registration views should reuse cache entries")
	}
}
