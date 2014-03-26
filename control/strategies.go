package control

import (
	"math"
	"sort"
)

// best fit: put job on machine with "smallest available hole"

// how to compute the hole size
type bestFitScoreMethod int

const (
	sumScoreMethod bestFitScoreMethod = iota
	sumSquareScoreMethod
	powerTenScoreMethod
)

type scoreSortFun func([]candHost) []candHost

var strategies map[bestFitScoreMethod]scoreSortFun

func init() {
	strategies = make(map[bestFitScoreMethod]scoreSortFun)

	strategies[sumScoreMethod] = sumScore
	strategies[sumSquareScoreMethod] = sumSquareScore
	strategies[powerTenScoreMethod] = powerTenScore
}

type bySumScore []candHost

func sum(h candHost) float64 {
	return h.mem + h.cores
}

func (a bySumScore) Len() int           { return len(a) }
func (a bySumScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a bySumScore) Less(i, j int) bool { return sum(a[i]) < sum(a[j]) }

func sumScore(lhs []candHost) []candHost {
	sort.Sort(bySumScore(lhs))
	return lhs
}

type bySumSquareScore []candHost

func sumSquare(h candHost) float64 {
	return h.mem*h.mem + h.cores*h.cores
}

func (a bySumSquareScore) Len() int           { return len(a) }
func (a bySumSquareScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a bySumSquareScore) Less(i, j int) bool { return sumSquare(a[i]) < sumSquare(a[j]) }

func sumSquareScore(lhs []candHost) []candHost {
	sort.Sort(bySumSquareScore(lhs))
	return lhs
}

type byPowerTenScore []candHost

func powerTen(h candHost) float64 {
	return math.Pow(10.0, h.mem) + math.Pow(10.0, h.cores)
}

func (a byPowerTenScore) Len() int           { return len(a) }
func (a byPowerTenScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPowerTenScore) Less(i, j int) bool { return powerTen(a[i]) < powerTen(a[j]) }

func powerTenScore(lhs []candHost) []candHost {
	sort.Sort(byPowerTenScore(lhs))
	return lhs
}
