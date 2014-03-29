package control

import (
	"fmt"
	"math/rand"
	"testing"
)

type mockUniformMachineDB struct{}

func (mumdb *mockUniformMachineDB) Spec(host string) (*MachineSpec, error) {
	return &MachineSpec{
		// 8 cores
		Cores: 800,
		// 32 gb ram
		Memory: 32768,
		// 1 tb disk
		LocalDiskSpace: 1000,
	}, nil
}

type mockHostAgent struct {
	host   string
	record map[string]string
	clus   *cluster
}

func (mag *mockHostAgent) RunJob(jid string, spec *JobSpec) error {
	mag.record[spec.Name] = mag.host
	if mag.clus != nil {
		mag.clus.JobScheduled(jid, mag.host, spec)
	}
	return nil
}

func newTestJob(index int, cores int, mem int, disk int) *JobSpec {
	return &JobSpec{
		Name:                   fmt.Sprintf("job%d", index),
		MemoryRequired:         mem,
		CoresRequired:          cores,
		LocalDiskSpaceRequired: disk,
	}
}

type mockEtcd struct {
	jwhs          []*JobWithHost
	hs            []string
	record        map[string]string
	clus          *cluster
	updateCluster bool
}

func (metcd *mockEtcd) declareJob(spec *JobSpec, host string) {
	metcd.jwhs = append(metcd.jwhs, &JobWithHost{
		Spec: spec,
		Host: host,
	})
}

func (metcd *mockEtcd) declareHost(host string) {
	metcd.hs = append(metcd.hs, host)
}

func (metcd *mockEtcd) AllJobs() ([]*JobWithHost, error) {
	return metcd.jwhs, nil
}

func (metcd *mockEtcd) AllHosts() ([]string, error) {
	return metcd.hs, nil
}

func (metcd *mockEtcd) HostAgent(host string) (HostAgent, error) {
	mag := &mockHostAgent{
		host:   host,
		record: metcd.record,
	}

	if metcd.updateCluster {
		mag.clus = metcd.clus
	}
	return mag, nil
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

	ctrl, err := NewJobControl(etcd, new(mockUniformMachineDB))
	if err != nil {
		fmt.Printf("could create job control: %v", err)
		return
	}

	ctrl.ScheduleJob(newTestJob(3, 200, 1024, 200))

	for k, v := range record {
		fmt.Printf("%s scheduled on %s", k, v)
	}
	// Output: job3 scheduled on host2
}

func newRandomJob(index int) *JobSpec {
	cores := 50 + rand.Intn(300)
	mem := 256 + rand.Intn(1024)
	disk := 10 + rand.Intn(100)

	return &JobSpec{
		Name:                   fmt.Sprintf("job%d", index),
		MemoryRequired:         mem,
		CoresRequired:          cores,
		LocalDiskSpaceRequired: disk,
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

	ctrl, err := NewJobControl(etcd, new(mockUniformMachineDB))
	if err != nil {
		fmt.Printf("could create job control: %v", err)
		return
	}

	etcd.clus = ctrl.(*cluster)
	etcd.updateCluster = true

	for i := 0; i < numJobs; i++ {
		_, err = ctrl.ScheduleJob(newRandomJob(i))
		if err != nil && err != ErrClusterFull {
			fmt.Printf("couldn't schedule job %d: %v", i, err)
			return
		}
	}

	spec := newRandomJob(numJobs)
	etcd.updateCluster = false

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err = ctrl.ScheduleJob(spec)
		if err != nil && err != ErrClusterFull {
			fmt.Printf("couldn't schedule job %d: %v", numJobs, err)
			return
		}
	}
}
