package execution

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/theQRL/go-qrl/common"
	"github.com/theQRL/go-qrl/rpc"
	pb "github.com/theQRL/qrysm/proto/engine/v1"
	"github.com/theQRL/qrysm/testing/require"
)

func bodyOfSize(size int) *pb.ExecutionPayloadBodyV1 {
	return &pb.ExecutionPayloadBodyV1{Transactions: [][]byte{make([]byte, size)}}
}

func TestPayloadBodyCache_EvictsLeastRecentlyUsed(t *testing.T) {
	cache := newPayloadBodyCache(100)
	cache.add(common.Hash{1}, bodyOfSize(40))
	cache.add(common.Hash{2}, bodyOfSize(40))
	_, ok := cache.get(common.Hash{1}) // 2 is now the least recently used
	require.Equal(t, true, ok)

	cache.add(common.Hash{3}, bodyOfSize(40))
	_, ok = cache.get(common.Hash{2})
	require.Equal(t, false, ok, "the least recently used body should be evicted")
	for _, hash := range []common.Hash{{1}, {3}} {
		_, ok = cache.get(hash)
		require.Equal(t, true, ok)
	}
	require.Equal(t, uint64(80), cache.size)
}

func TestPayloadBodyCache_SkipsBodyLargerThanCache(t *testing.T) {
	cache := newPayloadBodyCache(100)
	cache.add(common.Hash{1}, bodyOfSize(40))
	cache.add(common.Hash{2}, bodyOfSize(101))

	_, ok := cache.get(common.Hash{2})
	require.Equal(t, false, ok)
	_, ok = cache.get(common.Hash{1})
	require.Equal(t, true, ok, "a body too large to cache should not evict others")
}

func TestPayloadBodyCache_CountsWithdrawals(t *testing.T) {
	body := &pb.ExecutionPayloadBodyV1{
		Transactions: [][]byte{make([]byte, 10)},
		Withdrawals:  []*pb.Withdrawal{{}, {}},
	}
	require.Equal(t, uint64(10+2*withdrawalSize), payloadBodySize(body))
}

func TestPayloadBodyCache_NilHoldsNothing(t *testing.T) {
	var cache *payloadBodyCache
	cache.add(common.Hash{1}, bodyOfSize(1))
	_, ok := cache.get(common.Hash{1})
	require.Equal(t, false, ok)
}

func TestPayloadBodiesByHash_FetchesOnlyUncachedBodies(t *testing.T) {
	var requested [][]common.Hash
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Params [][]common.Hash `json:"params"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		requested = append(requested, request.Params[0])
		bodies := make([]map[string]any, len(request.Params[0]))
		for i := range bodies {
			bodies[i] = map[string]any{"transactions": []string{"0x02"}, "withdrawals": []any{}}
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": bodies}))
	}))
	defer srv.Close()
	rpcClient, err := rpc.Dial(srv.URL)
	require.NoError(t, err)
	defer rpcClient.Close()

	service := &Service{rpcClient: rpcClient, payloadBodies: newPayloadBodyCache(payloadBodyCacheSize)}
	cached := &pb.ExecutionPayloadBodyV1{Transactions: [][]byte{{0x01}}}
	service.payloadBodies.add(common.Hash{1}, cached)

	bodies, err := service.payloadBodiesByHash(context.Background(), []common.Hash{{1}, {2}})
	require.NoError(t, err)
	require.Equal(t, 1, len(requested))
	require.DeepEqual(t, []common.Hash{{2}}, requested[0])
	require.Equal(t, cached, bodies[0])
	require.DeepEqual(t, [][]byte{{0x02}}, bodies[1].Transactions)

	// The fetched body is cached too.
	_, err = service.payloadBodiesByHash(context.Background(), []common.Hash{{2}})
	require.NoError(t, err)
	require.Equal(t, 1, len(requested))
}
