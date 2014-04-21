package control

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/coreos/fleet/machine"
)

func someSpec() *machine.MachineSpec {
	return &machine.MachineSpec{
		// 8 cores
		Cores: 800,
		// 32 gb ram
		Memory: 32768,
		// 1 tb disk
		DiskSpace: 1000,
	}
}

func newTestJob(index int, cores int, mem int, disk int) *JobSpec {
	return &JobSpec{
		Name:              fmt.Sprintf("job%d", index),
		MemoryRequired:    mem,
		CoresRequired:     cores,
		DiskSpaceRequired: disk,
	}
}

type mockClusterCentral struct {
	jwhs   []*JobWithHost
	hs     []string
	record map[string]string
	clus   *cluster
}

func (mClusterCentral *mockClusterCentral) declareJob(spec *JobSpec, host string) {
	mClusterCentral.jwhs = append(mClusterCentral.jwhs, &JobWithHost{
		Spec:   spec,
		BootID: host,
	})
}

func (mClusterCentral *mockClusterCentral) declareHost(host string) {
	mClusterCentral.hs = append(mClusterCentral.hs, host)
}

func (mClusterCentral *mockClusterCentral) Jobs() ([]*JobWithHost, error) {
	return mClusterCentral.jwhs, nil
}

func (mClusterCentral *mockClusterCentral) Hosts() ([]string, error) {
	return mClusterCentral.hs, nil
}

func (mClusterCentral *mockClusterCentral) Spec(bootID string) (*machine.MachineSpec, error) {
	return someSpec(), nil
}

func (mClusterCentral *mockClusterCentral) Specs() (map[string]machine.MachineSpec, error) {
	r := make(map[string]machine.MachineSpec)
	for _, h := range mClusterCentral.hs {
		r[h] = *someSpec()
	}
	return r, nil
}

func ExampleScheduleJob() {
	record := make(map[string]string)

	clusterCentral := &mockClusterCentral{
		record: record,
	}

	clusterCentral.declareHost("host1")
	clusterCentral.declareHost("host2")
	clusterCentral.declareHost("host3")
	clusterCentral.declareHost("host4")

	clusterCentral.declareJob(newTestJob(1, 100, 1024, 10), "host1")
	clusterCentral.declareJob(newTestJob(2, 130, 2024, 100), "host2")

	ctrl, err := NewJobControl(clusterCentral)
	if err != nil {
		fmt.Printf("couldn't create job control: %v", err)
		return
	}

	bootIDs, err := ctrl.ScheduleJob(newTestJob(3, 200, 1024, 200))
	if err != nil {
		fmt.Printf("couldn't schedule job: %v", err)
		return
	}

	fmt.Printf("[%s]", strings.Join(bootIDs, ", "))

	// Output: [host2, host1, host3, host4]
}

func newRandomJob(index int) *JobSpec {
	cores := 50 + rand.Intn(300)
	mem := 256 + rand.Intn(1024)
	disk := 10 + rand.Intn(100)

	return &JobSpec{
		Name:              fmt.Sprintf("job%d", index),
		MemoryRequired:    mem,
		CoresRequired:     cores,
		DiskSpaceRequired: disk,
	}
}

func BenchmarkScheduleJob(b *testing.B) {
	// set up:
	// cluster with 10000 machines and 10000 jobs running

	numMachines := 10000
	numJobs := 10000

	record := make(map[string]string)

	clusterCentral := &mockClusterCentral{
		record: record,
	}

	for i := 0; i < numMachines; i++ {
		clusterCentral.declareHost(fmt.Sprintf("host%d", i))
	}

	ctrl, err := NewJobControl(clusterCentral)
	if err != nil {
		fmt.Printf("could create job control: %v", err)
		return
	}

	clusterCentral.clus = ctrl.(*cluster)

	for i := 0; i < numJobs; i++ {
		spec := newRandomJob(i)
		bootIDs, err := ctrl.ScheduleJob(spec)
		if err != nil && err != ErrClusterFull {
			fmt.Printf("couldn't schedule job %d: %v", i, err)
			return
		}

		if len(bootIDs) == 0 {
			fmt.Printf("couldn't schedule job %d: %v", i, err)
			return
		}

		clusterCentral.clus.jobScheduled(bootIDs[0], spec)
	}

	spec := newRandomJob(numJobs)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err = ctrl.ScheduleJob(spec)
		if err != nil && err != ErrClusterFull {
			fmt.Printf("couldn't schedule job %d: %v", numJobs, err)
			return
		}
	}
}
