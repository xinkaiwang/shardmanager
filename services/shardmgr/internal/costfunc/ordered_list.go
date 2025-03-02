package costfunc

type OrderedListItem interface {
	IsBetterThan(other OrderedListItem) bool
	Dropped() // get called when the item is dropped from the list
}

// OrderedList is a list that keeps items in order.
// The list has a max size, and will drop the last item if the list is full.
// The list is ordered from the best to the worst.
type OrderedList[T OrderedListItem] struct {
	maxSize int
	Items   []T
}

func NewOrderedList[T OrderedListItem](maxSize int) *OrderedList[T] {
	return &OrderedList[T]{
		maxSize: maxSize,
	}
}

// Add adds an item to the list, and returns true if the item is added successfully.
// If the list is full, the item will only be added if it's better than the worst item in the list.
// If the item is added, the worst item in the list will be removed.
func (ol *OrderedList[T]) Add(item T) bool {
	// If the list is not full, insert the item to where it should be.
	if len(ol.Items) < ol.maxSize {
		ol.insertItem(item)
		return true
	}
	// if the list is full, check if the item is better than the worst item in the list.
	lastItem := ol.Items[len(ol.Items)-1]
	if item.IsBetterThan(lastItem) {
		ol.insertItem(item)
		ol.Items = ol.Items[:len(ol.Items)-1]
		lastItem.Dropped()
		return true
	}
	return false
}

func (ol *OrderedList[T]) insertItem(item T) {
	for i, v := range ol.Items {
		if item.IsBetterThan(v) {
			ol.Items = append(ol.Items[:i], append([]T{item}, ol.Items[i:]...)...)
			return
		}
	}
	ol.Items = append(ol.Items, item)
}
