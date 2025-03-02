package costfunc

import "fmt"

// Cost evaluates the cost of current situation. it's always a positive number, the smaller the better.
type Cost struct {
	HardScore int32
	SoftScore float64
}

func (c Cost) IsLowerThan(other Cost) bool {
	if c.HardScore < other.HardScore {
		return true
	}
	if c.HardScore > other.HardScore {
		return false
	}
	return c.SoftScore < other.SoftScore
}

func (c Cost) IsEqualTo(other Cost) bool {
	return c.HardScore == other.HardScore && c.SoftScore == other.SoftScore
}

func (c Cost) IsGreaterThan(other Cost) bool {
	return !c.IsLowerThan(other) && !c.IsEqualTo(other)
}

// Gain: evaluates the benifit of a move. positive number means good move, the larger the better.
// Note: a good move is a move that reduces the cost.
type Gain struct {
	HardScore int32
	SoftScore float64
}

func (g Gain) IsGreaterThan(other Gain) bool {
	if g.HardScore > other.HardScore {
		return true
	}
	if g.HardScore < other.HardScore {
		return false
	}
	return g.SoftScore > other.SoftScore
}

func (g Gain) IsLowerThan(other Gain) bool {
	if g.HardScore < other.HardScore {
		return true
	}
	if g.HardScore > other.HardScore {
		return false
	}
	return g.SoftScore < other.SoftScore
}

func (g Gain) String() string {
	return fmt.Sprintf("{%d, %.3f}", g.HardScore, g.SoftScore)
}
