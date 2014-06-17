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

	"github.com/coreos/fleet/client"
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

	// global API client used by commands
	cAPI client.API

	// flags used by all commands
	globalFlags = struct {
		Debug                 bool
		Verbosity             int
		Version               bool
		Endpoint              string
		EtcdKeyPrefix         string
		UseAPI                bool
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
	globalFlagset.BoolVar(&globalFlags.UseAPI, "experimental-api", false, "Use the experimental HTTP API. This flag will be removed when the API is no longer considered experimental.")
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
	lv, err := cAPI.GetLatestVersion()
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

	if cmd.Name != "help" && cmd.Name != "version" {
		var err error
		cAPI, err = getClient()
		if err != nil {
			msg := fmt.Sprintf("Unable to initialize client: %v", err)
			fmt.Fprint(os.Stderr, msg)
			os.Exit(1)
		}

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

// getClient initializes a client of fleet based on CLI flags
func getClient() (client.API, error) {
	if globalFlags.UseAPI {
		return getHTTPClient()
	} else {
		return getRegistryClient(), nil
	}
}

func getHTTPClient() (client.API, error) {
	dialFunc := func(string, string) (net.Conn, error) {
		return net.Dial("unix", "/var/run/fleet.sock")
	}

	trans := http.Transport{
		Dial: dialFunc,
	}

	hc := http.Client{
		Transport: &trans,
	}

	return client.NewHTTPClient(&hc)
}

func getRegistryClient() client.API {
	var dial func(string, string) (net.Conn, error)
	tun := getTunnelFlag()
	if tun != "" {
		sshClient, err := ssh.NewSSHClient("core", tun, getChecker(), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed initializing SSH client: %v\n", err)
			os.Exit(1)
		}

		dial = func(network, addr string) (net.Conn, error) {
			tcpaddr, err := net.ResolveTCPAddr(network, addr)
			if err != nil {
				return nil, err
			}
			return sshClient.DialTCP(network, nil, tcpaddr)
		}
	}

	trans := http.Transport{
		Dial: dial,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	return client.NewRegistryClient(&trans, globalFlags.Endpoint, globalFlags.EtcdKeyPrefix)
}

// getChecker creates and returns a HostKeyChecker, or nil if any error is encountered
func getChecker() *ssh.HostKeyChecker {
	if !globalFlags.StrictHostKeyChecking {
		return nil
	}

	keyFile := ssh.NewHostKeyFile(globalFlags.KnownHostsFile)
	return ssh.NewHostKeyChecker(keyFile)
}

// getUnitFromFile attempts to load a Unit from a given filename
// It returns the Unit or nil, and any error encountered
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
		j, err := cAPI.GetJob(name)
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

	if err := cAPI.CreateJob(j); err != nil {
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

	err = cAPI.CreateSignatureSet(ss)
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

	ss, err := cAPI.GetSignatureSetOfJob(j.Name)
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

// lazyCreateJobs iterates over a set of Job names and, for each, attempts to
// ensure that a Job by that name exists in the Registry, by checking a number
// of conditions and acting on the first one that succeeds, in order of:
//  1. a Job by that name already existing in the Registry
//  2. a unit file by that name existing on disk
//  3. a corresponding unit template (if applicable) existing in the Registry
//  4. a corresponding unit template (if applicable) existing on disk
// Any error encountered during these steps is returned immediately (i.e.
// subsequent Jobs are not acted on). An error is also returned if none of the
// above conditions match a given Job.
// If signAndVerify is true, the Job will be signed (if it is created), or have
// its signature verified if it already exists in the Registry.
func lazyCreateJobs(args []string, signAndVerify bool) error {
	for _, arg := range args {
		// TODO(jonboulle): this loop is getting too unwieldy; factor it out

		jobName := unitNameMangle(arg)

		// First, check if there already exists a Job by the given name in the Registry
		j, err := cAPI.GetJob(jobName)
		if err != nil {
			return fmt.Errorf("error retrieving Job(%s) from Registry: %v", jobName, err)
		}
		if j != nil {
			log.V(1).Infof("Found Job(%s) in Registry, no need to recreate it", jobName)
			warnOnDifferentLocalUnit(arg, j)
			if signAndVerify {
				if err := verifyJob(j); err != nil {
					return err
				}
			}
			continue
		}

		// Failing that, assume the name references a local unit file on disk, and attempt to load that, if it exists
		if _, err := os.Stat(arg); !os.IsNotExist(err) {
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
			continue
		}

		// Otherwise (if the unit file does not exist), check if the name appears to be an instance unit,
		// and if so, check for a corresponding template unit in the Registry
		uni := unit.NewUnitNameInfo(jobName)
		if uni == nil {
			return fmt.Errorf("error extracting information from unit name %s", jobName)
		} else if !uni.IsInstance() {
			return fmt.Errorf("unable to find Unit(%s) in Registry or on filesystem", jobName)
		}
		tmpl, err := cAPI.GetJob(uni.Template)
		if err != nil {
			return fmt.Errorf("error retrieving template Job(%s) from Registry: %v", uni.Template, err)
		}

		// Finally, if we could not find a template unit in the Registry, check the local disk for one instead
		var u *unit.Unit
		if tmpl == nil {
			file := path.Join(path.Dir(arg), uni.Template)
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return fmt.Errorf("unable to find Unit(%s) or template Unit(%s) in Registry or on filesystem", jobName, uni.Template)
			}
			u, err = getUnitFromFile(file)
			if err != nil {
				return fmt.Errorf("failed getting template Unit(%s) from file: %v", uni.Template, err)
			}
		} else {
			warnOnDifferentLocalUnit(arg, tmpl)
			u = &tmpl.Unit
		}

		// If we found a template Unit or Job, create a near-identical instance Job in
		// the Registry - same Unit as the template, but different name
		j, err = createJob(jobName, u)
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

func warnOnDifferentLocalUnit(name string, j *job.Job) {
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		unit, err := getUnitFromFile(name)
		if err == nil && unit.Hash() != j.UnitHash {
			fmt.Fprintf(os.Stderr, "WARNING: Job(%s) in Registry differs from local Unit(%s)\n", j.Name, name)
			return
		}
	}
	if uni := unit.NewUnitNameInfo(path.Base(name)); uni != nil && uni.IsInstance() {
		file := path.Join(path.Dir(name), uni.Template)
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			tmpl, err := getUnitFromFile(file)
			if err == nil && tmpl.Hash() != j.UnitHash {
				fmt.Fprintf(os.Stderr, "WARNING: Job(%s) in Registry differs from local template Unit(%s)\n", j.Name, uni.Template)
			}
		}
	}
}

func lazyLoadJobs(args []string) ([]string, error) {
	jobs := make([]string, 0, len(args))
	for _, j := range args {
		jobs = append(jobs, unitNameMangle(j))
	}
	return setTargetStateOfJobs(jobs, job.JobStateLoaded)
}

func lazyStartJobs(args []string) ([]string, error) {
	jobs := make([]string, 0, len(args))
	for _, j := range args {
		jobs = append(jobs, unitNameMangle(j))
	}
	return setTargetStateOfJobs(jobs, job.JobStateLaunched)
}

// setTargetStateOfJobs ensures that the target state for the given Jobs is set
// to the given state in the Registry.
// On success, a slice of the Jobs for which a state change was made is returned.
// Any error encountered is immediately returned (i.e. this is not a transaction).
func setTargetStateOfJobs(jobs []string, state job.JobState) ([]string, error) {
	triggered := make([]string, 0)
	for _, name := range jobs {
		j, err := cAPI.GetJob(name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving Job(%s) from Registry: %v", name, err)
		} else if j == nil {
			return nil, fmt.Errorf("unable to find Job(%s)", name)
		} else if j.State == nil {
			return nil, fmt.Errorf("unable to determine current state of Job")
		} else if *(j.State) == state {
			log.V(1).Infof("Job(%s) already %s, skipping.", j.Name, *(j.State))
			continue
		}

		log.V(1).Infof("Setting Job(%s) target state to %s", j.Name, state)
		cAPI.SetJobTargetState(j.Name, state)
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
			if assertJobState(jobName, js, out) {
				return
			}
			time.Sleep(sleep)
		}
	} else {
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if assertJobState(jobName, js, out) {
				return
			}
			time.Sleep(sleep)
		}
		errchan <- fmt.Errorf("timed out waiting for job %s to report state %s", jobName, js)
	}
}

func assertJobState(name string, js job.JobState, out io.Writer) bool {
	j, err := cAPI.GetJob(name)
	if err != nil {
		log.Warningf("Error retrieving Job(%s) from Registry: %v", name, err)
		return false
	}
	if j == nil || j.State == nil || *(j.State) != js {
		return false
	}

	msg := fmt.Sprintf("Job %s %s", name, *(j.State))

	tgt, err := cAPI.GetJobTarget(name)
	if err != nil {
		log.Warningf("Error retrieving target information for Job(%s) from Registry: %v", name, err)
	} else if tgt != "" {
		if ms, _ := cAPI.GetMachineState(tgt); ms != nil {
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
