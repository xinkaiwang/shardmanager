package biz

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicorn"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

type UnicornBlitzApp struct {
	uc *unicorn.Unicorn
}

func NewUnicornBlitzApp(ctx context.Context) *UnicornBlitzApp {
	app := &UnicornBlitzApp{}
	builder := unicorn.NewUnicornBuilder()
	builder.WithRoutingTreeBuilder(func(workers map[data.WorkerId]*unicornjson.WorkerEntryJson) unicorn.RoutingTree {
		return unicorn.RangeBasedShardIdTreeBuilder(workers)
	})
	app.uc = builder.Build(ctx)
	return app
}

func (app *UnicornBlitzApp) GetTargetByShardingKey(ctx context.Context, shardingKey uint32) *unicorn.RoutingTarget {
	return app.uc.GetCurrentTree().FindShardByShardingKey(data.ShardingKey(shardingKey))
}

func (app *UnicornBlitzApp) GetTargetByObjectKey(ctx context.Context, objectId string) *unicorn.RoutingTarget {
	key := unicorn.JavaStringHashCode(objectId)
	return app.uc.GetCurrentTree().FindShardByShardingKey(data.ShardingKey(key))
}

func (app *UnicornBlitzApp) RunLoadTest(ctx context.Context, objId string) {
	shardingKey := unicorn.JavaStringHashCode(objId)
	target := app.GetTargetByShardingKey(ctx, shardingKey)
	if target == nil {
		panic("target is nil")
	}
	startMs := kcommon.GetWallTimeMs()
	var resp string
	ke := kcommon.TryCatchRun(ctx, func() {
		resp = CallShardPing(target.WorkerInfo.AddressPort, target.ShardId, objId)
	})
	elapsedMs := kcommon.GetWallTimeMs() - startMs
	if ke != nil {
		klogging.Error(ctx).With("shardingKey", Uint32ToHexString(shardingKey)).With("target", target.String()).With("error", ke).With("elapsedMs", elapsedMs).Log("UnicornBlitzApp", "RunLoadTest")
	} else {
		klogging.Debug(ctx).With("shardingKey", Uint32ToHexString(shardingKey)).With("objId", objId).With("target", target.String()).With("elapsedMs", elapsedMs).With("resp", resp).Log("UnicornBlitzApp", "RunLoadTest")
	}
}

func Uint32ToHexString(key uint32) string {
	// Convert uint32 to hex string
	hexString := fmt.Sprintf("%08x", key)
	return hexString
}

// return response body as string
func CallShardPing(portAddr string, shardId data.ShardId, objectId string) string {
	// make http call to portAddr
	url := fmt.Sprintf("http://%s/smg/ping?name=%s", portAddr, objectId)
	// make http call (with headers)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("X-Shard-Id", string(shardId))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ke := kerror.Wrap(err, "HttpCallError", "failed to call http", false)
		panic(ke)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		ke := kerror.Create("HttpCallError", "http call failed").With("statusCode", resp.StatusCode).With("url", url)
		panic(ke)
	}
	// readAll response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		ke := kerror.Wrap(err, "HttpCallError", "failed to read response body", false)
		// klogging.Error(ctx).With("error", ke).Log("HttpCallError", "failed to read response body")
		panic(ke)
	}
	return string(data)
}
