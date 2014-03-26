package control

import (
	"fmt"
	"testing"
)

type mockUniformMachineDB struct{}

func (mumdb *mockUniformMachineDB) Spec(host HostID) (*MachineSpec, error) {
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
	host   HostID
	record map[string]HostID
}

func (mag *mockHostAgent) RunJob(user UserID, jid JobID, spec *JobSpec) error {
	mag.record[spec.Name] = mag.host
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
	jwhs   []*JobWithHost
	hs     []HostID
	record map[string]HostID
}

func (metcd *mockEtcd) declareJob(spec *JobSpec, host HostID) {
	metcd.jwhs = append(metcd.jwhs, &JobWithHost{
		Spec: spec,
		Host: host,
	})
}

func (metcd *mockEtcd) declareHost(host HostID) {
	metcd.hs = append(metcd.hs, host)
}

func (metcd *mockEtcd) AllJobs() ([]*JobWithHost, error) {
	return metcd.jwhs, nil
}

func (metcd *mockEtcd) AllHosts() ([]HostID, error) {
	return metcd.hs, nil
}

func (metcd *mockEtcd) HostAgent(host HostID) (HostAgent, error) {
	mag := &mockHostAgent{
		host:   host,
		record: metcd.record,
	}
	return mag, nil
}

func TestControl(t *testing.T) {
	record := make(map[string]HostID)

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
		t.Fatalf("could create job control: %v", err)
	}

	ctrl.ScheduleJob("user1", newTestJob(3, 200, 1024, 200))

	for k, v := range record {
		t.Logf("%s scheduled on %s", k, v)
	}
}
