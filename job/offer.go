package job

import (
	"fmt"
	"strings"
)

type JobOffer struct {
	Job          Job
	Peers        []string
	Requirements map[string][]string
}

func NewOfferFromJob(j Job) *JobOffer {
	peers := []string{}
	if j.Type != "systemd-service" {
		idx := strings.LastIndex(j.Name, ".")
		if idx != -1 {
			svc := fmt.Sprintf("%s.%s", j.Name[0:idx], "service")
			peers = append(peers, svc)
		}
	}
	return &JobOffer{j, peers, j.Requirements}
}

type JobBid struct {
	JobName     string
	MachineName string
}

func NewBid(jobName string, machName string) *JobBid {
	return &JobBid{jobName, machName}
}
