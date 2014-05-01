package machine

import (
	"bufio"
	"bytes"
	"os/exec"
	"runtime"
	"strconv"
)

type MachineSpec struct {
	// in hundreds, ie 100=1core, 50=0.5core, 200=2cores, etc
	Cores int
	// in MB
	Memory int
	// in MB
	DiskSpace int
}

func ReadLocalSpec() (*MachineSpec, error) {
	spec := new(MachineSpec)
	spec.Cores = 100 * runtime.NumCPU()

	// TODO(uwedeportivo): determine disk space

	mem, err := readMemory()
	if err != nil {
		return nil, err
	}
	spec.Memory = mem
	spec.DiskSpace = 1024

	return spec, nil
}

func parseFreeMOutput(memstr []byte) (int, error) {
	ss := bufio.NewScanner(bytes.NewBuffer(memstr))
	ss.Split(bufio.ScanWords)
	seenMemToken := false
	mem := 0
	for ss.Scan() {
		token := ss.Text()
		if seenMemToken {
			m, err := strconv.Atoi(token)
			if err != nil {
				return 0, err
			}
			mem = m
			break
		} else if token == "Mem:" {
			seenMemToken = true
		}
	}
	if err := ss.Err(); err != nil {
		return 0, err
	}
	return mem, nil
}

func readMemory() (int, error) {
	memstr, err := exec.Command("free", "-m").Output()
	if err != nil {
		return 0, err
	}
	return parseFreeMOutput(memstr)
}
