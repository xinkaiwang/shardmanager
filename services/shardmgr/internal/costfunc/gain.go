package costfunc

import "fmt"

// Cost evaluates the cost of current situation. it's always a positive number, the smaller the better.
type Cost struct {
	HardScore int32
	SoftScore float64
}

func NewCost(hardScore int32, softScore float64) Cost {
	return Cost{
		HardScore: hardScore,
		SoftScore: softScore,
	}
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

func (c Cost) Substract(other Cost) Gain {
	return Gain{
		HardScore: c.HardScore - other.HardScore,
		SoftScore: c.SoftScore - other.SoftScore,
	}
}

func (c Cost) String() string {
	return fmt.Sprintf("{%d, %.3f}", c.HardScore, c.SoftScore)
}

// Gain: evaluates the benifit of a move. positive number means good move, the larger the better.
// Note: a good move is a move that reduces the cost.
type Gain struct {
	HardScore int32
	SoftScore float64
}

func NewGain(hardScore int32, softScore float64) Gain {
	return Gain{
		HardScore: hardScore,
		SoftScore: softScore,
	}
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

func (g Gain) Add(other Gain) Gain {
	return Gain{
		HardScore: g.HardScore + other.HardScore,
		SoftScore: g.SoftScore + other.SoftScore,
	}
}

func (g Gain) String() string {
	return fmt.Sprintf("{%d, %.3f}", g.HardScore, g.SoftScore)
}
