package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
)

// ServiceStateVisitorEvent implements IEvent interface
type ServiceStateVisitorEvent struct {
	createTimeMs int64 // time when the event was created
	callback     func(*ServiceState)
}

func NewServiceStateVisitorEvent(callback func(*ServiceState)) *ServiceStateVisitorEvent {
	return &ServiceStateVisitorEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
		callback:     callback,
	}
}
func (e *ServiceStateVisitorEvent) GetCreateTimeMs() int64 {
	return e.createTimeMs
}
func (e *ServiceStateVisitorEvent) GetName() string {
	return "ServiceStateVisitor"
}
func (e *ServiceStateVisitorEvent) Process(ctx context.Context, ss *ServiceState) {
	e.callback(ss)
}

func (ss *ServiceState) VisitServiceState(ctx context.Context, callback func(*ServiceState)) {
	ss.PostEvent(NewServiceStateVisitorEvent(callback))
}

func (ss *ServiceState) VisitServiceStateAndWait(ctx context.Context, callback func(*ServiceState)) {
	ch := make(chan struct{})
	eve := NewServiceStateVisitorEvent(func(ss *ServiceState) {
		callback(ss)
		close(ch)
	})
	ss.PostEvent(eve)
	<-ch
}
