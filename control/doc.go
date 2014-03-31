/*
Package control schedules jobs in a cluster. Scheduling happens according to
current cluster load along three coordinates (memory, CPU cores and local disk space).

Basics

NewJobControl creates a job control that is able to schedule jobs.

Job control knows the current load in the cluster and knows what the requirements are
for the job it tries to schedule. It picks a host satisfying the job requirements
from the list of available and active hosts.

If more than one host can run the job, then job control uses a strategy to choose
between the candidates. The problem of choosing the best host is closely related to
the http://en.wikipedia.org/wiki/Bin_packing_problem.

A good overview of online bin packing algorithms can be found at
http://i11www.iti.uni-karlsruhe.de/_media/teaching/sommer2010/approximationsonlinealgorithmen/onl-bp.pdf.

Best Fit

The only strategy currently implemented is Best Fit. It tries to fit the job on a host
leaving the smallest hole of remaining free resources. This avoids resource fragmentation.
There are several methods available on how to determine the size of the hole, ie combining
the free memory, free core slices and free disk space into one number representing the remaining
free space on a host.

Current Load

Job control needs to keep track of what happens in the cluster. There could be additional job controls operating
in the same cluster, scheduling jobs too. Therefore job control needs to listen for job scheduled, job downed,
host activated and host downed events.

When a job control gets created, it needs to catch up to the current load situation in the cluster. It will use
Etcd to ask about all the active hosts and all the jobs runnning on them.

Host Agent

When job control has picked a host to schedule a job on it, it will talk to the hosts HostAgent. This is an http/rpc
interface into the host agent running locally on the chosen host. Job control asks host agent to run the job. Host agent
can refuse to do so if it determines that job requirements are not satisfied anymore since the host was chosen or that
it has other reasons to refuse to run the job.
*/
package control
