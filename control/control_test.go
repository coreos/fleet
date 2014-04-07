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

type mockEtcd struct {
	jwhs   []*JobWithHost
	hs     []string
	record map[string]string
	clus   *cluster
}

func (metcd *mockEtcd) declareJob(spec *JobSpec, host string) {
	metcd.jwhs = append(metcd.jwhs, &JobWithHost{
		Spec:   spec,
		BootID: host,
	})
}

func (metcd *mockEtcd) declareHost(host string) {
	metcd.hs = append(metcd.hs, host)
}

func (metcd *mockEtcd) Jobs() ([]*JobWithHost, error) {
	return metcd.jwhs, nil
}

func (metcd *mockEtcd) Hosts() ([]string, error) {
	return metcd.hs, nil
}

func (metcd *mockEtcd) Spec(bootID string) (*machine.MachineSpec, error) {
	return someSpec(), nil
}

func (metcd *mockEtcd) Specs() (map[string]machine.MachineSpec, error) {
	r := make(map[string]machine.MachineSpec)
	for _, h := range metcd.hs {
		r[h] = *someSpec()
	}
	return r, nil
}

func ExampleScheduleJob() {
	record := make(map[string]string)

	etcd := &mockEtcd{
		record: record,
	}

	etcd.declareHost("host1")
	etcd.declareHost("host2")
	etcd.declareHost("host3")
	etcd.declareHost("host4")

	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host1")
	etcd.declareJob(newTestJob(2, 130, 2024, 100), "host2")

	ctrl, err := NewJobControl(etcd)
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

	etcd := &mockEtcd{
		record: record,
	}

	for i := 0; i < numMachines; i++ {
		etcd.declareHost(fmt.Sprintf("host%d", i))
	}

	ctrl, err := NewJobControl(etcd)
	if err != nil {
		fmt.Printf("could create job control: %v", err)
		return
	}

	etcd.clus = ctrl.(*cluster)

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

		etcd.clus.jobScheduled(bootIDs[0], spec)
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
