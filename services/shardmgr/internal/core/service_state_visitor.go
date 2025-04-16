package core

import "context"

// ServiceStateVisitorEvent implements IEvent interface
type ServiceStateVisitorEvent struct {
	callback func(*ServiceState)
}

func NewServiceStateVisitorEvent(callback func(*ServiceState)) *ServiceStateVisitorEvent {
	return &ServiceStateVisitorEvent{
		callback: callback,
	}
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
