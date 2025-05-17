package unicorn

import (
	"context"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/etcdprov"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
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
	klogging.Info(ctx).With("etcdEndpoints", etcdEndpoints).Log("NewUnicorn", "LoadAllByPrefix first load...")
	list, rev := uc.etcdClient.LoadAllByPrefix(ctx, routingTablePathPrefix)
	klogging.Info(ctx).With("etcdEndpoints", etcdEndpoints).With("count", len(list)).Log("NewUnicorn", "LoadAllByPrefix done")
	for _, item := range list {
		entry := unicornjson.WorkerEntryJsonFromJson(item.Value)
		workerId := data.WorkerId(entry.WorkerId)
		uc.stagingArea[workerId] = entry
	}
	// digest the routing table
	uc.digistRoutingEvents(ctx, uc)
	uc.watcher = NewUnicornWatcher(ctx, uc, routingTablePathPrefix, rev)
	klogging.Info(ctx).With("etcdEndpoints", etcdEndpoints).Log("NewUnicorn", "finished")
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
	visitor func(uc *Unicorn)
}

func (eve *UnicornVisitorEvent) GetName() string {
	return "UnicornVisitorEvent"
}
func (eve *UnicornVisitorEvent) Process(ctx context.Context, resource *Unicorn) {
	eve.visitor(resource)
}
func NewUnicornVisitorEvent(visitor func(uc *Unicorn)) *UnicornVisitorEvent {
	return &UnicornVisitorEvent{
		visitor: visitor,
	}
}
