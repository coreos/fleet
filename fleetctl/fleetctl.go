package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	log "github.com/coreos/fleet/third_party/github.com/golang/glog"

	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/sign"
	"github.com/coreos/fleet/ssh"
	"github.com/coreos/fleet/unit"
	"github.com/coreos/fleet/version"
)

const (
	cliName        = "fleetctl"
	cliDescription = "fleetctl is a command-line interface to fleet, the cluster-wide CoreOS init system."

	oldVersionWarning = `####################################################################
WARNING: fleetctl (%s) is older than the latest registered
version of fleet found in the cluster (%s). You are strongly
recommended to upgrade fleetctl to prevent incompatibility issues.
####################################################################
`
)

var (
	out           *tabwriter.Writer
	globalFlagset = flag.NewFlagSet("fleetctl", flag.ExitOnError)

	// set of top-level commands
	commands []*Command

	// global Registry used by commands
	registryCtl registry.Registry

	// flags used by all commands
	globalFlags = struct {
		Debug                 bool
		Verbosity             int
		Version               bool
		Endpoint              string
		EtcdKeyPrefix         string
		KnownHostsFile        string
		StrictHostKeyChecking bool
		Tunnel                string
	}{}

	// flags used by multiple commands
	sharedFlags = struct {
		Sign          bool
		Full          bool
		NoLegend      bool
		NoBlock       bool
		BlockAttempts int
		Fields        string
	}{}
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr. Equivalent to --verbosity=1")
	globalFlagset.BoolVar(&globalFlags.Version, "version", false, "Print the version and exit")
	globalFlagset.IntVar(&globalFlags.Verbosity, "verbosity", 0, "Log at a specified level of verbosity to stderr.")
	globalFlagset.StringVar(&globalFlags.Endpoint, "endpoint", "http://127.0.0.1:4001", "etcd endpoint for fleet")
	globalFlagset.StringVar(&globalFlags.EtcdKeyPrefix, "etcd-key-prefix", registry.DefaultKeyPrefix, "Keyspace for fleet data in etcd (development use only!)")
	globalFlagset.StringVar(&globalFlags.KnownHostsFile, "known-hosts-file", ssh.DefaultKnownHostsFile, "File used to store remote machine fingerprints. Ignored if strict host key checking is disabled.")
	globalFlagset.BoolVar(&globalFlags.StrictHostKeyChecking, "strict-host-key-checking", true, "Verify host keys presented by remote machines before initiating SSH connections.")
	globalFlagset.StringVar(&globalFlags.Tunnel, "tunnel", "", "Establish an SSH tunnel through the provided address for communication with fleet and etcd.")
}

type Command struct {
	Name        string       // Name of the Command and the string to use to invoke it
	Summary     string       // One-sentence summary of what the Command does
	Usage       string       // Usage options/arguments
	Description string       // Detailed description of command
	Flags       flag.FlagSet // Set of flags associated with this command

	Run func(args []string) int // Run a command with the given arguments, return exit status

}

func init() {
	out = new(tabwriter.Writer)
	out.Init(os.Stdout, 0, 8, 1, '\t', 0)
	commands = []*Command{
		cmdCatUnit,
		cmdDebugInfo,
		cmdDestroyUnit,
		cmdHelp,
		cmdJournal,
		cmdListMachines,
		cmdListUnits,
		cmdLoadUnits,
		cmdSSH,
		cmdStartUnit,
		cmdStatusUnits,
		cmdStopUnit,
		cmdSubmitUnit,
		cmdUnloadUnit,
		cmdVerifyUnit,
		cmdVersion,
	}
}

func getAllFlags() (flags []*flag.Flag) {
	return getFlags(globalFlagset)
}

func getFlags(flagset *flag.FlagSet) (flags []*flag.Flag) {
	flags = make([]*flag.Flag, 0)
	flagset.VisitAll(func(f *flag.Flag) {
		flags = append(flags, f)
	})
	return
}

// checkVersion makes a best-effort attempt to verify that fleetctl is at least as new as the
// latest fleet version found registered in the cluster. If any errors are encountered or fleetctl
// is >= the latest version found, it returns true. If it is < the latest found version, it returns
// false and a scary warning to the user.
func checkVersion() (string, bool) {
	fv := version.SemVersion
	lv, err := registryCtl.GetLatestVersion()
	if err != nil {
		log.Errorf("error attempting to check latest fleet version in Registry: %v", err)
	} else if lv != nil && fv.LessThan(*lv) {
		return fmt.Sprintf(oldVersionWarning, fv.String(), lv.String()), false
	}
	return "", true
}

func main() {
	// parse global arguments
	globalFlagset.Parse(os.Args[1:])

	var args = globalFlagset.Args()

	getFlagsFromEnv(cliName, globalFlagset)

	if globalFlags.Debug && globalFlags.Verbosity < 1 {
		globalFlags.Verbosity = 1
	}

	// configure glog, which uses the global command line options
	if globalFlags.Verbosity > 0 {
		flag.CommandLine.Lookup("v").Value.Set(strconv.Itoa(globalFlags.Verbosity))
		flag.CommandLine.Lookup("logtostderr").Value.Set("true")
	}

	// no command specified - trigger help
	if len(args) < 1 {
		args = append(args, "help")
	}

	// deal specially with --version
	if globalFlags.Version {
		args[0] = "version"
	}

	var cmd *Command

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
				fmt.Println(err.Error())
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		fmt.Printf("%v: unknown subcommand: %q\n", cliName, args[0])
		fmt.Printf("Run '%v help' for usage.\n", cliName)
		os.Exit(2)
	}

	// TODO(jonboulle): increase cleverness of registry initialization
	if cmd.Name != "help" && cmd.Name != "version" {
		registryCtl = getRegistry()
		msg, ok := checkVersion()
		if !ok {
			fmt.Fprint(os.Stderr, msg)
		}
	}

	os.Exit(cmd.Run(cmd.Flags.Args()))

}

// getFlagsFromEnv parses all registered flags in the given flagset,
// and if they are not already set it attempts to set their values from
// environment variables. Environment variables take the name of the flag but
// are UPPERCASE, have the given prefix, and any dashes are replaced by
// underscores - for example: some-flag => PREFIX_SOME_FLAG
func getFlagsFromEnv(prefix string, fs *flag.FlagSet) {
	alreadySet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		alreadySet[f.Name] = true
	})
	fs.VisitAll(func(f *flag.Flag) {
		if !alreadySet[f.Name] {
			key := strings.ToUpper(prefix + "_" + strings.Replace(f.Name, "-", "_", -1))
			val := os.Getenv(key)
			if val != "" {
				fs.Set(f.Name, val)
			}
		}

	})
}

// getRegistry initializes a connection to the Registry
func getRegistry() registry.Registry {
	tun := getTunnelFlag()

	machines := []string{globalFlags.Endpoint}
	client := etcd.NewClient(machines)

	if tun != "" {
		sshClient, err := ssh.NewSSHClient("core", tun, getChecker(), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed initializing SSH client: %v\n", err)
			os.Exit(1)
		}

		dial := func(network, addr string) (net.Conn, error) {
			tcpaddr, err := net.ResolveTCPAddr(network, addr)
			if err != nil {
				return nil, err
			}
			return sshClient.DialTCP(network, nil, tcpaddr)
		}

		tr := http.Transport{
			Dial: dial,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		client.SetTransport(&tr)
	}

	return registry.New(client, globalFlags.EtcdKeyPrefix)
}

// getChecker creates and returns a HostKeyChecker, or nil if any error is encountered
func getChecker() *ssh.HostKeyChecker {
	if !globalFlags.StrictHostKeyChecking {
		return nil
	}

	keyFile := ssh.NewHostKeyFile(globalFlags.KnownHostsFile)
	return ssh.NewHostKeyChecker(keyFile)
}

// getUnitFromFile attempts to load a Job from a given filename
// It returns the Job or nil, and any error encountered
func getUnitFromFile(file string) (*unit.Unit, error) {
	out, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	unitName := path.Base(file)
	log.V(1).Infof("Unit(%s) found in local filesystem", unitName)

	return unit.NewUnit(string(out)), nil
}

func getTunnelFlag() string {
	tun := globalFlags.Tunnel
	if tun != "" && !strings.Contains(tun, ":") {
		tun += ":22"
	}
	return tun
}

func machineIDLegend(ms machine.MachineState, full bool) string {
	legend := ms.ID
	if !full {
		legend = fmt.Sprintf("%s...", ms.ShortID())
	}
	return legend
}

func machineFullLegend(ms machine.MachineState, full bool) string {
	legend := machineIDLegend(ms, full)
	if len(ms.PublicIP) > 0 {
		legend = fmt.Sprintf("%s/%s", legend, ms.PublicIP)
	}
	return legend
}

func findJobs(args []string) (jobs []job.Job, err error) {
	jobs = make([]job.Job, len(args))
	for i, v := range args {
		name := unitNameMangle(v)
		j, err := registryCtl.GetJob(name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving Job(%s) from Registry: %v", name, err)
		} else if j == nil {
			return nil, fmt.Errorf("could not find Job(%s)", name)
		}

		jobs[i] = *j
	}

	return jobs, nil
}

func createJob(jobName string, unit *unit.Unit) (*job.Job, error) {
	j := job.NewJob(jobName, *unit)

	if err := registryCtl.CreateJob(j); err != nil {
		return nil, fmt.Errorf("failed creating job %s: %v", j.Name, err)
	}

	log.V(1).Infof("Created Job(%s) in Registry", j.Name)

	return j, nil
}

// signJob signs the Unit of a Job using the public keys in the local SSH
// agent, and pushes the resulting SignatureSet to the Registry
func signJob(j *job.Job) error {
	sc, err := sign.NewSignatureCreatorFromSSHAgent()
	if err != nil {
		return fmt.Errorf("failed creating SignatureCreator: %v", err)
	}

	ss, err := sc.SignJob(j)
	if err != nil {
		return fmt.Errorf("failed signing Job(%s): %v", j.Name, err)
	}

	err = registryCtl.CreateSignatureSet(ss)
	if err != nil {
		return fmt.Errorf("failed storing Job signature in registry: %v", err)
	}

	log.V(1).Infof("Signed Job(%s)", j.Name)
	return nil
}

// verifyJob attempts to verify the signature of the provided Job's unit using
// the public keys in the local SSH agent
func verifyJob(j *job.Job) error {
	sv, err := sign.NewSignatureVerifierFromSSHAgent()
	if err != nil {
		return fmt.Errorf("failed creating SignatureVerifier: %v", err)
	}

	ss, err := registryCtl.GetSignatureSetOfJob(j.Name)
	if err != nil {
		return fmt.Errorf("failed attempting to retrieve SignatureSet of Job(%s): %v", j.Name, err)
	}
	verified, err := sv.VerifyJob(j, ss)
	if err != nil {
		return fmt.Errorf("failed attempting to verify Job(%s): %v", j.Name, err)
	} else if !verified {
		return fmt.Errorf("unable to verify Job(%s)", j.Name)
	}

	log.V(1).Infof("Verified signature of Job(%s)", j.Name)
	return nil
}

func lazyCreateJobs(args []string, signAndVerify bool) error {
	for _, arg := range args {
		jobName := unitNameMangle(arg)
		j, err := registryCtl.GetJob(jobName)
		if err != nil {
			log.V(1).Infof("Error retrieving Job(%s) from Registry: %v", jobName, err)
			continue
		}
		if j != nil {
			log.V(1).Infof("Found Job(%s) in Registry, no need to recreate it", jobName)
			if signAndVerify {
				if err := verifyJob(j); err != nil {
					return err
				}
			}
			continue
		}

		unit, err := getUnitFromFile(arg)
		if err != nil {
			return fmt.Errorf("failed getting Unit(%s) from file: %v", jobName, err)
		}

		j, err = createJob(jobName, unit)
		if err != nil {
			return err
		}

		if signAndVerify {
			if err := signJob(j); err != nil {
				return err
			}
		}
	}
	return nil
}

func lazyLoadJobs(args []string) ([]string, error) {
	triggered := make([]string, 0)
	for _, v := range args {
		name := unitNameMangle(v)
		j, err := registryCtl.GetJob(name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving Job(%s) from Registry: %v", name, err)
		} else if j == nil || j.State == nil {
			return nil, fmt.Errorf("unable to determine state of job %s", name)
		} else if *(j.State) == job.JobStateLoaded || *(j.State) == job.JobStateLaunched {
			log.V(1).Infof("Job(%s) already %s, skipping.", j.Name, *(j.State))
			continue
		}

		log.V(1).Infof("Setting Job(%s) target state to loaded", j.Name)
		registryCtl.SetJobTargetState(j.Name, job.JobStateLoaded)
		triggered = append(triggered, j.Name)
	}

	return triggered, nil
}

func lazyStartJobs(args []string) ([]string, error) {
	triggered := make([]string, 0)
	for _, v := range args {
		name := unitNameMangle(v)
		j, err := registryCtl.GetJob(name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving Job(%s) from Registry: %v", name, err)
		} else if j == nil {
			return nil, fmt.Errorf("unable to find Job(%s)", name)
		} else if j.State == nil {
			return nil, fmt.Errorf("unable to determine current state of Job")
		}

		ts := job.JobStateLaunched
		if j.IsBatch() {
			ts = job.JobStateCompleted
		}

		if *(j.State) == ts {
			log.V(1).Infof("Job(%s) already %s, skipping.", j.Name, *(j.State))
			continue
		}

		log.V(1).Infof("Setting Job(%s) target state to %s", j.Name, ts)
		registryCtl.SetJobTargetState(j.Name, ts)
		triggered = append(triggered, j.Name)
	}

	return triggered, nil
}

// waitForJobStates polls each of the indicated jobs until each of their
// states is equal to that which the caller indicates, or until the
// polling operation times out. waitForJobStates will retry forever, or
// up to maxAttempts times before timing out if maxAttempts is greater
// than zero. Returned is an error channel used to communicate when
// timeouts occur. The returned error channel will be closed after all
// polling operation is complete.
func waitForJobStates(jobs []string, js job.JobState, maxAttempts int, out io.Writer) chan error {
	errchan := make(chan error)
	var wg sync.WaitGroup
	for _, name := range jobs {
		wg.Add(1)
		go checkJobState(name, js, maxAttempts, out, &wg, errchan)
	}

	go func() {
		wg.Wait()
		close(errchan)
	}()

	return errchan
}

func checkJobState(jobName string, js job.JobState, maxAttempts int, out io.Writer, wg *sync.WaitGroup, errchan chan error) {
	defer wg.Done()

	sleep := 100 * time.Millisecond

	if maxAttempts < 1 {
		for {
			if assertJobState(jobName, js) {
				return
			}
			time.Sleep(sleep)
		}
	} else {
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if assertJobState(jobName, js) {
				return
			}
			time.Sleep(sleep)
		}
		errchan <- fmt.Errorf("timed out waiting for job %s to report state %s", jobName, js)
	}
}

func assertJobState(name string, js job.JobState) bool {
	j, err := registryCtl.GetJob(name)
	if err != nil {
		log.Warningf("Error retrieving Job(%s) from Registry: %v", name, err)
		return false
	}
	if j == nil || j.State == nil || *(j.State) != js {
		return false
	}

	msg := fmt.Sprintf("Job %s %s", name, *(j.State))

	tgt, err := registryCtl.GetJobTarget(name)
	if err != nil {
		log.Warningf("Error retrieving target information for Job(%s) from Registry: %v", name, err)
	} else if tgt != "" {
		if ms, _ := registryCtl.GetMachineState(tgt); ms != nil {
			msg = fmt.Sprintf("%s on %s", msg, machineFullLegend(*ms, false))
		}
	}

	fmt.Fprintln(out, msg)
	return true
}

// unitNameMangle tries to turn a string that might not be a unit name into a
// sensible unit name.
func unitNameMangle(baseName string) string {
	name := path.Base(baseName)

	if !unit.RecognizedUnitType(name) {
		return unit.DefaultUnitType(name)
	}

	return name
}
