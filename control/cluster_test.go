package control

import "testing"

func checkLoad(host string, mload MachineSpec, cores, memory, disk int, t *testing.T) {
	if mload.Cores != cores {
		t.Errorf("host %s: expected %d cores load, got %d", host, cores, mload.Cores)
	}

	if mload.Memory != memory {
		t.Errorf("host %s: expected %d memory load, got %d", host, memory, mload.Memory)
	}

	if mload.LocalDiskSpace != disk {
		t.Errorf("host %s: expected %d disk load, got %d", host, disk, mload.LocalDiskSpace)
	}
}

func TestJobScheduled(t *testing.T) {
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

	mdb := new(mockUniformMachineDB)

	ctrl, err := NewJobControl(etcd, mdb)
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

	etcd := &mockEtcd{
		record: record,
	}

	etcd.declareHost("host1")
	etcd.declareHost("host2")
	etcd.declareHost("host3")
	etcd.declareHost("host4")

	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host1")
	etcd.declareJob(newTestJob(2, 130, 2024, 100), "host2")

	mdb := new(mockUniformMachineDB)

	ctrl, err := NewJobControl(etcd, mdb)
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

	etcd := &mockEtcd{
		record: record,
	}

	etcd.declareHost("host1")
	etcd.declareHost("host2")
	etcd.declareHost("host3")
	etcd.declareHost("host4")

	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host1")
	etcd.declareJob(newTestJob(2, 130, 2024, 100), "host2")

	mdb := new(mockUniformMachineDB)

	ctrl, err := NewJobControl(etcd, mdb)
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

	etcd := &mockEtcd{
		record: record,
	}

	etcd.declareHost("host1")
	etcd.declareHost("host2")
	etcd.declareHost("host3")
	etcd.declareHost("host4")

	etcd.declareJob(newTestJob(1, 100, 1024, 10), "host1")
	etcd.declareJob(newTestJob(2, 130, 2024, 100), "host2")

	mdb := new(mockUniformMachineDB)

	ctrl, err := NewJobControl(etcd, mdb)
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

func testCandidates(clus *cluster, spec *JobSpec, expected []candHost, expectedError error, t *testing.T) {
	hosts, err := clus.candidates(spec)
	if err != nil && err != expectedError {
		t.Fatalf("error computing candidates: %v", err)
	}
	if !candHostSlicesEqual(hosts, expected) {
		t.Errorf("candidates: %s, expected %s, got %s", spec.Name, ctoS(expected), ctoS(hosts))
	}
}

func TestCandidates(t *testing.T) {
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

	mdb := new(mockUniformMachineDB)

	ctrl, err := NewJobControl(etcd, mdb)
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
