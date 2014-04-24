package control

import (
	"sort"
	"testing"

	"github.com/coreos/fleet/machine"
)

func checkLoad(host string, mload machine.MachineSpec, cores, memory, disk int, t *testing.T) {
	if mload.Cores != cores {
		t.Errorf("host %s: expected %d cores load, got %d", host, cores, mload.Cores)
	}

	if mload.Memory != memory {
		t.Errorf("host %s: expected %d memory load, got %d", host, memory, mload.Memory)
	}

	if mload.DiskSpace != disk {
		t.Errorf("host %s: expected %d disk load, got %d", host, disk, mload.DiskSpace)
	}
}

func TestJobScheduled(t *testing.T) {
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
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	clus.JobScheduled("j0003", "host1", newTestJob(3, 50, 2048, 100))

	checkLoad("host1", clus.loads["host1"], 150, 3072, 110, t)
	checkLoad("host2", clus.loads["host2"], 130, 2024, 100, t)
	checkLoad("host3", clus.loads["host3"], 0, 0, 0, t)
	checkLoad("host4", clus.loads["host4"], 0, 0, 0, t)

	clus.JobScheduled("j0006", "host5", newTestJob(1, 100, 2048, 100))

	checkLoad("host1", clus.loads["host1"], 150, 3072, 110, t)
	checkLoad("host2", clus.loads["host2"], 130, 2024, 100, t)
	checkLoad("host3", clus.loads["host3"], 0, 0, 0, t)
	checkLoad("host4", clus.loads["host4"], 0, 0, 0, t)

	checkLoad("host5", clus.loads["host5"], 100, 2048, 100, t)
}

func TestJobDowned(t *testing.T) {
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
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	clus.JobDowned("j0003", "host1", newTestJob(1, 100, 1024, 10))

	checkLoad("host1", clus.loads["host1"], 0, 0, 0, t)
	checkLoad("host2", clus.loads["host2"], 130, 2024, 100, t)
	checkLoad("host3", clus.loads["host3"], 0, 0, 0, t)
	checkLoad("host4", clus.loads["host4"], 0, 0, 0, t)
}

func TestHostUp(t *testing.T) {
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
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	clus.HostUp("host5")

	checkLoad("host1", clus.loads["host1"], 100, 1024, 10, t)
	checkLoad("host2", clus.loads["host2"], 130, 2024, 100, t)
	checkLoad("host3", clus.loads["host3"], 0, 0, 0, t)
	checkLoad("host4", clus.loads["host4"], 0, 0, 0, t)

	checkLoad("host5", clus.loads["host5"], 0, 0, 0, t)
}

func TestHostDown(t *testing.T) {
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
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	clus.HostDown("host1")

	checkLoad("host1", clus.loads["host1"], 0, 0, 0, t)
	checkLoad("host2", clus.loads["host2"], 130, 2024, 100, t)
	checkLoad("host3", clus.loads["host3"], 0, 0, 0, t)
	checkLoad("host4", clus.loads["host4"], 0, 0, 0, t)
}

type byName []candHost

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].host < a[j].host }

func testCandidates(clus *cluster, spec *JobSpec, expected []candHost, expectedError error, t *testing.T) {
	hosts, err := clus.candidates(spec)
	if err != nil && err != expectedError {
		t.Fatalf("error computing candidates: %v", err)
	}

	sort.Sort(byName(hosts))
	if !candHostSlicesEqual(hosts, expected) {
		t.Errorf("candidates: %s, expected %s, got %s", spec.Name, ctoS(expected), ctoS(hosts))
	}
}

func TestCandidates(t *testing.T) {
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
		t.Fatalf("could create job control: %v", err)
	}

	clus := ctrl.(*cluster)

	testCandidates(clus, newTestJob(3, 100, 1024, 10), candidateHosts([]string{"host1", "host2", "host3", "host4"}), nil, t)
	testCandidates(clus, newTestJob(4, 700, 1024, 10), candidateHosts([]string{"host1", "host3", "host4"}), nil, t)
	testCandidates(clus, newTestJob(5, 700, 31744, 10), candidateHosts([]string{"host1", "host3", "host4"}), nil, t)
	testCandidates(clus, newTestJob(6, 700, 31745, 10), candidateHosts([]string{"host3", "host4"}), nil, t)
	testCandidates(clus, newTestJob(7, 700, 32768, 10), candidateHosts([]string{"host3", "host4"}), nil, t)
	testCandidates(clus, newTestJob(8, 900, 32768, 10), nil, ErrClusterFull, t)
}
