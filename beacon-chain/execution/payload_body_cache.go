package execution

import (
	"container/list"
	"sync"

	"github.com/theQRL/go-qrl/common"
	pb "github.com/theQRL/qrysm/proto/engine/v1"
)

// payloadBodyCacheSize is how many bytes of payload bodies the cache holds.
// Serving blocks to peers rebuilds blinded blocks from these bodies, and peers
// that sync the same range ask for the same blocks again. A full payload is
// several MB, so this keeps the last couple of dozen full ones.
const payloadBodyCacheSize = 128 * 1024 * 1024

// withdrawalSize is the approximate memory one withdrawal takes in a body.
const withdrawalSize = 64

// payloadBodyCache is an LRU cache of execution payload bodies by block hash,
// bounded by the bodies' total size. A nil cache holds nothing.
type payloadBodyCache struct {
	lock    sync.Mutex
	items   map[common.Hash]*list.Element
	order   *list.List // most recently used first
	size    uint64
	maxSize uint64
}

type payloadBodyEntry struct {
	hash common.Hash
	body *pb.ExecutionPayloadBodyV1
	size uint64
}

func newPayloadBodyCache(maxSize uint64) *payloadBodyCache {
	return &payloadBodyCache{
		items:   make(map[common.Hash]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

// payloadBodySize returns the approximate memory a payload body takes.
func payloadBodySize(body *pb.ExecutionPayloadBodyV1) uint64 {
	size := uint64(len(body.Withdrawals)) * withdrawalSize
	for _, tx := range body.Transactions {
		size += uint64(len(tx))
	}
	return size
}

// get returns the body of the execution block with the given hash, if cached.
func (c *payloadBodyCache) get(hash common.Hash) (*pb.ExecutionPayloadBodyV1, bool) {
	if c == nil {
		return nil, false
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	element, ok := c.items[hash]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(element)
	return element.Value.(*payloadBodyEntry).body, true
}

// add caches the body of the execution block with the given hash, evicting the
// least recently used bodies until it fits. A body larger than the whole cache
// is not cached. The body must not be modified afterwards.
func (c *payloadBodyCache) add(hash common.Hash, body *pb.ExecutionPayloadBodyV1) {
	if c == nil || body == nil {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()

	if element, ok := c.items[hash]; ok {
		c.order.MoveToFront(element)
		return
	}
	size := payloadBodySize(body)
	if size > c.maxSize {
		return
	}
	for c.size+size > c.maxSize {
		oldest := c.order.Back()
		if oldest == nil {
			break
		}
		entry := c.order.Remove(oldest).(*payloadBodyEntry)
		delete(c.items, entry.hash)
		c.size -= entry.size
	}
	c.items[hash] = c.order.PushFront(&payloadBodyEntry{hash: hash, body: body, size: size})
	c.size += size
}
