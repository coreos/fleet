package control

import (
	"math"
	"sort"
)

// Resources:

// http://en.wikipedia.org/wiki/Bin_packing_problem
// we have an online bin packing problem,
// where items are not known upfront,
// so cannot be sorted before packing

// Good overview:
// http://i11www.iti.uni-karlsruhe.de/_media/teaching/sommer2010/approximationsonlinealgorithmen/onl-bp.pdf

// best fit: put job on machine with "smallest available hole"
// heuristic for this: best fit preserves machine space for
// subsequent larger jobs that in other strategies would face
// fragmentation and cannot be scheduled

// how to compute the hole size
type bestFitScoreMethod int

const (
	sumScoreMethod bestFitScoreMethod = iota
	sumSquareScoreMethod
	powerTenScoreMethod
)

type scoreFun func(candHost) float64

var strategies map[bestFitScoreMethod]scoreFun

func init() {
	strategies = make(map[bestFitScoreMethod]scoreFun)

	strategies[sumScoreMethod] = sumScore
	strategies[sumSquareScoreMethod] = sumSquareScore
	strategies[powerTenScoreMethod] = powerTenScore
}

type byScore []candHost

func (a byScore) Len() int           { return len(a) }
func (a byScore) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byScore) Less(i, j int) bool { return a[i].score < a[j].score }

func sortBestFit(lhs []candHost, scoreMethod bestFitScoreMethod) {
	sf := strategies[scoreMethod]

	for i, h := range lhs {
		lhs[i].score = sf(h)
	}

	sort.Sort(byScore(lhs))
}

func sumScore(h candHost) float64 {
	return h.mem + h.cores
}

func sumSquareScore(h candHost) float64 {
	return h.mem*h.mem + h.cores*h.cores
}

func powerTenScore(h candHost) float64 {
	return math.Pow(10.0, h.mem) + math.Pow(10.0, h.cores)
}
