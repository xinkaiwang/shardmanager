package unicorn

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/etcdprov"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
)

const (
	routingTablePathPrefix = "/smg/routing/"
)

type Unicorn struct {
	etcdEndpoints       []string
	treeBuilder         RoutingTreeBuilder
	currentTree         atomic.Value
	runloop             *krunloop.RunLoop[*Unicorn]
	etcdClient          etcdprov.EtcdProvider
	currentRputingTable map[data.WorkerId]*unicornjson.WorkerEntryJson

	batchMgr    *BatchManager
	stagingArea map[data.WorkerId]*unicornjson.WorkerEntryJson

	watcher *UnicornWatcher
}

func NewUnicorn(ctx context.Context, etcdEndpoints []string, treeBuilder RoutingTreeBuilder) *Unicorn {
	uc := &Unicorn{
		etcdEndpoints:       etcdEndpoints,
		treeBuilder:         treeBuilder,
		etcdClient:          etcdprov.GetCurrentEtcdProvider(ctx),
		currentRputingTable: make(map[data.WorkerId]*unicornjson.WorkerEntryJson),
		stagingArea:         make(map[data.WorkerId]*unicornjson.WorkerEntryJson),
	}
	uc.runloop = krunloop.NewRunLoop(ctx, uc, "unicorn")
	go uc.runloop.Run(ctx)
	uc.batchMgr = NewBatchManager(uc.runloop, 100, "BatchDigist", uc.digistRoutingEvents)
	// Initialize the current tree
	slog.InfoContext(ctx, "load all by prefix first load",
		slog.String("event", "NewUnicorn.LoadStart"),
		slog.Any("etcdEndpoints", etcdEndpoints))
	list, rev := uc.etcdClient.LoadAllByPrefix(ctx, routingTablePathPrefix)
	slog.InfoContext(ctx, "load all by prefix done",
		slog.String("event", "NewUnicorn.LoadDone"),
		slog.Any("etcdEndpoints", etcdEndpoints),
		slog.Int("count", len(list)))
	for _, item := range list {
		entry := unicornjson.WorkerEntryJsonFromJson(item.Value)
		workerId := data.WorkerId(entry.WorkerId)
		uc.stagingArea[workerId] = entry
	}
	// digest the routing table
	uc.digistRoutingEvents(ctx, uc)
	uc.watcher = NewUnicornWatcher(ctx, uc, routingTablePathPrefix, rev)
	slog.InfoContext(ctx, "new unicorn finished",
		slog.String("event", "NewUnicorn.Finished"),
		slog.Any("etcdEndpoints", etcdEndpoints))
	return uc
}

func (uc *Unicorn) IsResource() {}

func (uc *Unicorn) setCurrentTree(tree RoutingTree) {
	uc.currentTree.Store(tree)
}

func (uc *Unicorn) GetCurrentTree() RoutingTree {
	tree := uc.currentTree.Load()
	if tree == nil {
		return nil
	}
	return tree.(RoutingTree)
}

func (uc *Unicorn) PostRoutingEvent(ctx context.Context, eve *RoutingEvent) {
	uc.runloop.PostEvent(eve)
}

func (uc *Unicorn) digistRoutingEvents(ctx context.Context, resource *Unicorn) {
	// update the routing table
	for workerId, entry := range resource.stagingArea {
		if entry == nil {
			delete(uc.currentRputingTable, workerId)
			continue
		}
		resource.currentRputingTable[workerId] = entry
	}
	uc.stagingArea = make(map[data.WorkerId]*unicornjson.WorkerEntryJson)
	// Build the routing tree
	tree := uc.treeBuilder(resource.currentRputingTable)
	uc.setCurrentTree(tree)
}

func (uc *Unicorn) VisitUnicorn(visitor func(uc *Unicorn)) {
	eve := NewUnicornVisitorEvent(visitor)
	uc.runloop.PostEvent(eve)
}

func (uc *Unicorn) VisitUnicornAndWait(ctx context.Context, visitor func(*Unicorn)) {
	ch := make(chan struct{})
	eve := NewUnicornVisitorEvent(func(uc *Unicorn) {
		visitor(uc)
		close(ch)
	})
	uc.runloop.PostEvent(eve)
	// Wait for the event to be processed
	<-ch
}

type UnicornVisitorEvent struct {
	createTimeMs int64 // time when the event was created
	visitor      func(uc *Unicorn)
}

func (eve *UnicornVisitorEvent) GetCreateTimeMs() int64 {
	return eve.createTimeMs
}
func (eve *UnicornVisitorEvent) GetName() string {
	return "UnicornVisitorEvent"
}
func (eve *UnicornVisitorEvent) Process(ctx context.Context, resource *Unicorn) {
	eve.visitor(resource)
}
func NewUnicornVisitorEvent(visitor func(uc *Unicorn)) *UnicornVisitorEvent {
	return &UnicornVisitorEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		visitor:      visitor,
	}
}
