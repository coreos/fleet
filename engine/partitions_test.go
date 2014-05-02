package engine

import (
	"fmt"
	"sort"
	"testing"

	"github.com/coreos/fleet/machine"
)

func machName(c int) string {
	return fmt.Sprintf("boot%d", c)
}

func jobName(c int) string {
	return fmt.Sprintf("job%d", c)
}

func createMachines(num int) []machine.MachineState {
	ms := make([]machine.MachineState, num)

	for i := 0; i < num; i++ {
		ms[i].ID = machName(i)
	}
	return ms
}

func createCluster(jd []int) *cluster {
	cu := newCluster()

	jc := 0
	for mi, nj := range jd {
		mach := machName(mi)
		cu.populateMachine(mach)
		for i := 0; i < nj; i++ {
			cu.populateJob(jobName(jc), mach)
			jc++
		}
	}

	return cu
}

func verifyCluster(clust *cluster, ejd []int, t *testing.T) {
	for i := 0; i < len(ejd); i++ {
		mach := machName(i)
		if clust.machineJobCount[mach] != ejd[i]+1 {
			t.Errorf("expected %d jobs on machine %s, got %d", ejd[i]+1, mach, clust.machineJobCount[mach])
		}
	}

	numJobs := 0
	partials := make([]int, len(ejd))
	for i, v := range ejd {
		numJobs += v
		partials[i] = numJobs
	}

	for i := 0; i < numJobs; i++ {
		m := sort.SearchInts(partials, i+1)
		if clust.jobsToMachines[jobName(i)] != machName(m) {
			t.Errorf("expected job %s on machine %s, got %s", jobName(i), machName(m), clust.jobsToMachines[jobName(i)])
		}
	}
}

func TestLeastLoaded(t *testing.T) {
	expectedJobDistribution := []int{1, 4, 3, 5, 5, 3, 0, 2, 6, 3}
	clust := createCluster(expectedJobDistribution)
	clust.upToDate = true

	least := clust.kLeastLoaded(3)

	if len(least) != 3 {
		t.Errorf("expected len of least 3, got %d", len(least))
	}

	sort.Strings(least)

	expectedLeast := []string{"boot0", "boot6", "boot7"}

	for i := 0; i < 3; i++ {
		if least[i] != expectedLeast[i] {
			t.Errorf("expected machine %s, got %s", expectedLeast[i], least[i])
		}
	}
}

func TestRefreshFrom(t *testing.T) {
	clust := newCluster()
	if clust.isUpToDate() {
		t.Errorf("newly created cluster cannot be up to date")
	}

	// mma: RandomVariate[PoissonDistribution[3], 10]
	expectedJobDistribution := []int{6, 3, 4, 2, 4, 1, 3, 6, 1, 2}

	cu := createCluster(expectedJobDistribution)

	clust.refreshFrom(cu, false)
	if !clust.isUpToDate() {
		t.Errorf("refreshed cluster is not up to date")
	}

	verifyCluster(clust, expectedJobDistribution, t)
}

func TestClusterKeepsUpToDate(t *testing.T) {
	clust := newCluster()
	expectedJobDistribution := []int{4, 1, 0, 4, 1, 4, 2, 7, 1, 5}
	clust.upToDate = true

	ms := createMachines(len(expectedJobDistribution))
	for _, mach := range ms {
		clust.populateMachine(mach.ID)
	}

	jc := 0
	for mi, nj := range expectedJobDistribution {
		for i := 0; i < nj; i++ {
			clust.jobScheduled(jobName(jc), &ms[mi])
			jc++
		}
	}
	verifyCluster(clust, expectedJobDistribution, t)

	clust.jobStopped(jobName(4))
	clust.jobStopped(jobName(7))

	if clust.machineJobCount[machName(1)] != 1 {
		t.Errorf("expected 1 jobs on machine 1, got %d", clust.machineJobCount[machName(1)])
	}

	if clust.machineJobCount[machName(3)] != 4 {
		t.Errorf("expected 4 jobs on machine 3, got %d", clust.machineJobCount[machName(3)])
	}
}
