package common

import "context"

type OrderedListItem interface {
	IsBetterThan(other OrderedListItem) bool
	Dropped(ctx context.Context, reason EnqueueResult) // get called when the item is dropped from the list
	GetSignature() string
}

// OrderedList is a list that keeps items in order.
// The list has a max size, and will drop the last item if the list is full.
// The list is ordered from the best to the worst.
type OrderedList[T OrderedListItem] struct {
	maxSize    int
	Items      []T
	signatures map[string]Unit
}

func NewOrderedList[T OrderedListItem](maxSize int) *OrderedList[T] {
	return &OrderedList[T]{
		maxSize:    maxSize,
		signatures: make(map[string]Unit),
	}
}

type EnqueueResult string

const (
	ER_Enqueued  EnqueueResult = "enqueued"
	ER_LowGain   EnqueueResult = "low_gain"
	ER_Duplicate EnqueueResult = "duplicate"
	ER_Accepted  EnqueueResult = "accepted"
	ER_Conflict  EnqueueResult = "conflict"
)

// Add adds an item to the list, and returns true if the item is added successfully.
// If the list is full, the item will only be added if it's better than the worst item in the list.
// If the item is added, the worst item in the list will be removed.
func (ol *OrderedList[T]) Enqueue(ctx context.Context, item T) EnqueueResult {
	sig := item.GetSignature()
	if _, ok := ol.signatures[sig]; ok {
		return ER_Duplicate
	}

	ol.insertItem(item)
	ol.signatures[sig] = Unit{}

	if len(ol.Items) <= ol.maxSize {
		return ER_Enqueued
	}

	// need to drop the last item
	last := ol.Items[len(ol.Items)-1]
	lastSig := last.GetSignature()
	ol.Items = ol.Items[:len(ol.Items)-1]

	if lastSig == sig {
		delete(ol.signatures, sig)
		return ER_LowGain
	} else {
		delete(ol.signatures, lastSig)
		last.Dropped(ctx, ER_LowGain)
		return ER_Enqueued
	}
}

func (ol *OrderedList[T]) Peak() T {
	return ol.Items[0]
}

func (ol *OrderedList[T]) Pop() T {
	item := ol.Items[0]
	ol.Items = ol.Items[1:]
	delete(ol.signatures, item.GetSignature())
	return item
}

func (ol *OrderedList[T]) insertItem(item T) {
	// first append to the last, and see whether it's better than the previous one, until it's not
	ol.Items = append(ol.Items, item)
	for i := len(ol.Items) - 1; i > 0; i-- {
		if ol.Items[i].IsBetterThan(ol.Items[i-1]) {
			ol.Items[i], ol.Items[i-1] = ol.Items[i-1], ol.Items[i]
		} else {
			break
		}
	}
}
