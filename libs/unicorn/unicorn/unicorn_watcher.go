package unicorn

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/etcdprov"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

// RoutingEvent implements IEvent interface
type RoutingEvent struct {
	createTimeMs int64 // time when the event was created
	key          string
	data         *unicornjson.WorkerEntryJson
}

func (e *RoutingEvent) GetCreateTimeMs() int64 {
	return e.createTimeMs
}
func (e *RoutingEvent) GetName() string {
	return "RoutingEvent"
}

func (e *RoutingEvent) Process(ctx context.Context, resource *Unicorn) {
	if e.data == nil {
		// delete
		klogging.Info(ctx).With("key", e.key).Log("RoutingEvent", "删除路由表项")
		workerId := data.WorkerId(e.key)
		delete(resource.currentRputingTable, workerId)
		resource.batchMgr.TryScheduleInternal(ctx, "workerDelete:"+e.key)
	} else {
		// update
		klogging.Info(ctx).With("key", e.key).With("data", e.data).Log("RoutingEvent", "更新路由表项")
		workerId := data.WorkerId(e.key)
		resource.stagingArea[workerId] = e.data
		resource.batchMgr.TryScheduleInternal(ctx, "workerUpdated:"+e.key)
	}
}

func NewRoutingEvent(key string, data *unicornjson.WorkerEntryJson) *RoutingEvent {
	return &RoutingEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		key:          key,
		data:         data,
	}
}

type UnicornWatcher struct {
	parent *Unicorn
	ch     chan etcdprov.EtcdKvItem
}

func NewUnicornWatcher(ctx context.Context, parent *Unicorn, path string, revision etcdprov.EtcdRevision) *UnicornWatcher {
	watcher := &UnicornWatcher{
		parent: parent,
	}
	watcher.ch = etcdprov.GetCurrentEtcdProvider(ctx).WatchByPrefix(ctx, path, revision)
	go watcher.Run(ctx)
	return watcher
}

func (w *UnicornWatcher) Run(ctx context.Context) {
	for {
		select {
		case item := <-w.ch:
			klogging.Info(ctx).With("key", item.Key).With("value", item.Value).Log("UnicornWatcher", "观察到路由表已更新")
			if item.Key == "" {
				continue
			}
			if item.Value == "" {
				// delete
				w.parent.PostRoutingEvent(ctx, NewRoutingEvent(item.Key[len(routingTablePathPrefix):], nil))
			} else {
				// update
				w.parent.PostRoutingEvent(ctx, NewRoutingEvent(item.Key[len(routingTablePathPrefix):], unicornjson.WorkerEntryJsonFromJson(item.Value)))
			}
		case <-ctx.Done():
			return
		}
	}
}
