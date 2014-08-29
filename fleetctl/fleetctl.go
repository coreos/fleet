package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/coreos/fleet/client"
	"github.com/coreos/fleet/etcd"
	"github.com/coreos/fleet/job"
	"github.com/coreos/fleet/log"
	"github.com/coreos/fleet/machine"
	"github.com/coreos/fleet/pkg"
	"github.com/coreos/fleet/registry"
	"github.com/coreos/fleet/schema"
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
		Version               bool
		Endpoint              string
		EtcdKeyPrefix         string
		EtcdKeyFile           string
		EtcdCertFile          string
		EtcdCAFile            string
		UseAPI                bool
		KnownHostsFile        string
		StrictHostKeyChecking bool
		Tunnel                string
		RequestTimeout        float64
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

	// used to cache MachineStates
	machineStates map[string]*machine.MachineState
)

func init() {
	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr")
	globalFlagset.BoolVar(&globalFlags.Version, "version", false, "Print the version and exit")
	globalFlagset.StringVar(&globalFlags.Endpoint, "endpoint", "http://127.0.0.1:4001", "etcd endpoint for fleet")
	globalFlagset.StringVar(&globalFlags.EtcdKeyPrefix, "etcd-key-prefix", registry.DefaultKeyPrefix, "Keyspace for fleet data in etcd (development use only!)")
	globalFlagset.StringVar(&globalFlags.EtcdKeyFile, "etcd-keyfile", "", "etcd key file authentication")
	globalFlagset.StringVar(&globalFlags.EtcdCertFile, "etcd-certfile", "", "etcd cert file authentication")
	globalFlagset.StringVar(&globalFlags.EtcdCAFile, "etcd-cafile", "", "etcd CA file authentication")
	globalFlagset.BoolVar(&globalFlags.UseAPI, "experimental-api", false, "Use the experimental HTTP API. This flag will be removed when the API is no longer considered experimental.")
	globalFlagset.StringVar(&globalFlags.KnownHostsFile, "known-hosts-file", ssh.DefaultKnownHostsFile, "File used to store remote machine fingerprints. Ignored if strict host key checking is disabled.")
	globalFlagset.BoolVar(&globalFlags.StrictHostKeyChecking, "strict-host-key-checking", true, "Verify host keys presented by remote machines before initiating SSH connections.")
	globalFlagset.StringVar(&globalFlags.Tunnel, "tunnel", "", "Establish an SSH tunnel through the provided address for communication with fleet and etcd.")
	globalFlagset.Float64Var(&globalFlags.RequestTimeout, "request-timeout", 3.0, "Amount of time in seconds to allow a single request before considering it failed.")
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
		cmdListUnitFiles,
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
func checkVersion(reg registry.Registry) (string, bool) {
	fv := version.SemVersion
	lv, err := reg.LatestVersion()
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

	if globalFlags.Debug {
		log.SetVerbosity(1)
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

	if sharedFlags.Sign {
		fmt.Fprintln(os.Stderr, "WARNING: The signed/verified units feature is DEPRECATED and cannot be used.")
		os.Exit(2)
	}

	if cmd.Name != "help" && cmd.Name != "version" {
		var err error
		cAPI, err = getClient()
		if err != nil {
			msg := fmt.Sprintf("Unable to initialize client: %v\n", err)
			fmt.Fprint(os.Stderr, msg)
			os.Exit(1)
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
		return getRegistryClient()
	}
}

func getHTTPClient() (client.API, error) {
	dialDomainSocket := strings.HasPrefix(globalFlags.Endpoint, "/")

	tunnelFunc := net.Dial
	tun := getTunnelFlag()
	if tun != "" {
		sshClient, err := ssh.NewSSHClient("core", tun, getChecker(), false)
		if err != nil {
			return nil, fmt.Errorf("failed initializing SSH client: %v", err)
		}

		tunnelFunc = func(net, addr string) (net.Conn, error) {
			return sshClient.Dial(net, addr)
		}
	}

	dialFunc := tunnelFunc
	if dialDomainSocket {
		dialFunc = func(net, addr string) (net.Conn, error) {
			return tunnelFunc("unix", globalFlags.Endpoint)
		}
	}

	trans := pkg.LoggingHTTPTransport{
		http.Transport{
			Dial: dialFunc,
		},
	}

	hc := http.Client{
		Transport: &trans,
	}

	endpoint := globalFlags.Endpoint
	if dialDomainSocket {
		endpoint = "http://domain-sock/"
	} else if !strings.HasPrefix(endpoint, "http") {
		endpoint = fmt.Sprintf("http://%s", endpoint)
	}

	return client.NewHTTPClient(&hc, endpoint)
}

func getRegistryClient() (client.API, error) {
	var dial func(string, string) (net.Conn, error)
	tun := getTunnelFlag()
	if tun != "" {
		sshClient, err := ssh.NewSSHClient("core", tun, getChecker(), false)
		if err != nil {
			return nil, fmt.Errorf("failed initializing SSH client: %v", err)
		}

		dial = func(network, addr string) (net.Conn, error) {
			tcpaddr, err := net.ResolveTCPAddr(network, addr)
			if err != nil {
				return nil, err
			}
			return sshClient.DialTCP(network, nil, tcpaddr)
		}
	}

	tlsConfig, err := etcd.ReadTLSConfigFiles(globalFlags.EtcdCAFile, globalFlags.EtcdCertFile, globalFlags.EtcdKeyFile)
	if err != nil {
		return nil, err
	}

	trans := &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
	}

	timeout := time.Duration(globalFlags.RequestTimeout*1000) * time.Millisecond
	machines := []string{globalFlags.Endpoint}
	eClient, err := etcd.NewClient(machines, trans, timeout)
	if err != nil {
		return nil, err
	}

	reg := registry.New(eClient, globalFlags.EtcdKeyPrefix)

	if msg, ok := checkVersion(reg); !ok {
		fmt.Fprint(os.Stderr, msg)
	}

	return &client.RegistryClient{reg}, nil
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
func getUnitFromFile(file string) (*unit.UnitFile, error) {
	out, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	unitName := path.Base(file)
	log.V(1).Infof("Unit(%s) found in local filesystem", unitName)

	return unit.NewUnitFile(string(out))
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

func findUnits(args []string) (sus []schema.Unit, err error) {
	units, err := cAPI.Units()
	if err != nil {
		return nil, err
	}

	uMap := make(map[string]*schema.Unit, len(units))
	for _, u := range units {
		u := u
		uMap[u.Name] = u
	}

	filtered := make([]schema.Unit, 0)
	for _, v := range args {
		v = unitNameMangle(v)
		u, ok := uMap[v]
		if !ok {
			continue
		}
		filtered = append(filtered, *u)
	}

	return filtered, nil
}

func createUnit(name string, uf *unit.UnitFile) (*schema.Unit, error) {
	u := schema.Unit{
		Name:    name,
		Options: schema.MapUnitFileToSchemaUnitOptions(uf),
	}
	err := cAPI.CreateUnit(&u)
	if err != nil {
		return nil, fmt.Errorf("failed creating unit %s: %v", name, err)
	}

	log.V(1).Infof("Created Unit(%s) in Registry", name)
	return &u, nil
}

// lazyCreateUnits iterates over a set of unit names and, for each, attempts to
// ensure that a unit by that name exists in the Registry, by checking a number
// of conditions and acting on the first one that succeeds, in order of:
//  1. a unit by that name already existing in the Registry
//  2. a unit file by that name existing on disk
//  3. a corresponding unit template (if applicable) existing in the Registry
//  4. a corresponding unit template (if applicable) existing on disk
// Any error encountered during these steps is returned immediately (i.e.
// subsequent Jobs are not acted on). An error is also returned if none of the
// above conditions match a given Job.
func lazyCreateUnits(args []string) error {
	for _, arg := range args {
		// TODO(jonboulle): this loop is getting too unwieldy; factor it out

		name := unitNameMangle(arg)

		// First, check if there already exists a Unit by the given name in the Registry
		u, err := cAPI.Unit(name)
		if err != nil {
			return fmt.Errorf("error retrieving Unit(%s) from Registry: %v", name, err)
		}
		if u != nil {
			log.V(1).Infof("Found Unit(%s) in Registry, no need to recreate it", name)
			warnOnDifferentLocalUnit(arg, u)
			continue
		}

		// Failing that, assume the name references a local unit file on disk, and attempt to load that, if it exists
		if _, err := os.Stat(arg); !os.IsNotExist(err) {
			unit, err := getUnitFromFile(arg)
			if err != nil {
				return fmt.Errorf("failed getting Unit(%s) from file: %v", name, err)
			}
			u, err = createUnit(name, unit)
			if err != nil {
				return err
			}
			continue
		}

		// Otherwise (if the unit file does not exist), check if the name appears to be an instance unit,
		// and if so, check for a corresponding template unit in the Registry
		uni := unit.NewUnitNameInfo(name)
		if uni == nil {
			return fmt.Errorf("error extracting information from unit name %s", name)
		} else if !uni.IsInstance() {
			return fmt.Errorf("unable to find Unit(%s) in Registry or on filesystem", name)
		}
		tmpl, err := cAPI.Unit(uni.Template)
		if err != nil {
			return fmt.Errorf("error retrieving template Unit(%s) from Registry: %v", uni.Template, err)
		}

		// Finally, if we could not find a template unit in the Registry, check the local disk for one instead
		var uf *unit.UnitFile
		if tmpl == nil {
			file := path.Join(path.Dir(arg), uni.Template)
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return fmt.Errorf("unable to find Unit(%s) or template Unit(%s) in Registry or on filesystem", name, uni.Template)
			}
			uf, err = getUnitFromFile(file)
			if err != nil {
				return fmt.Errorf("failed getting template Unit(%s) from file: %v", uni.Template, err)
			}
		} else {
			warnOnDifferentLocalUnit(arg, tmpl)
			uf = schema.MapSchemaUnitOptionsToUnitFile(tmpl.Options)
		}

		// If we found a template unit, create a near-identical instance unit in
		// the Registry - same unit file as the template, but different name
		u, err = createUnit(name, uf)
		if err != nil {
			return err
		}
	}
	return nil
}

func warnOnDifferentLocalUnit(name string, su *schema.Unit) {
	suf := schema.MapSchemaUnitOptionsToUnitFile(su.Options)
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		luf, err := getUnitFromFile(name)
		if err == nil && luf.Hash() != suf.Hash() {
			fmt.Fprintf(os.Stderr, "WARNING: Unit %s in registry differs from local unit file %s\n", su.Name, name)
			return
		}
	}
	if uni := unit.NewUnitNameInfo(path.Base(name)); uni != nil && uni.IsInstance() {
		file := path.Join(path.Dir(name), uni.Template)
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			tmpl, err := getUnitFromFile(file)
			if err == nil && tmpl.Hash() != suf.Hash() {
				fmt.Fprintf(os.Stderr, "WARNING: Unit %s in registry differs from local template unit file %s\n", su.Name, uni.Template)
			}
		}
	}
}

func lazyLoadUnits(args []string) ([]*schema.Unit, error) {
	units := make([]string, 0, len(args))
	for _, j := range args {
		units = append(units, unitNameMangle(j))
	}
	return setTargetStateOfUnits(units, job.JobStateLoaded)
}

func lazyStartUnits(args []string) ([]*schema.Unit, error) {
	units := make([]string, 0, len(args))
	for _, j := range args {
		units = append(units, unitNameMangle(j))
	}
	return setTargetStateOfUnits(units, job.JobStateLaunched)
}

// setTargetStateOfUnits ensures that the target state for the given Units is set
// to the given state in the Registry.
// On success, a slice of the Units for which a state change was made is returned.
// Any error encountered is immediately returned (i.e. this is not a transaction).
func setTargetStateOfUnits(units []string, state job.JobState) ([]*schema.Unit, error) {
	triggered := make([]*schema.Unit, 0)
	for _, name := range units {
		u, err := cAPI.Unit(name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving unit %s from registry: %v", name, err)
		} else if u == nil {
			return nil, fmt.Errorf("unable to find unit %s", name)
		} else if job.JobState(u.DesiredState) == state {
			log.V(1).Infof("Unit(%s) already %s, skipping.", u.Name, u.DesiredState)
			continue
		}

		log.V(1).Infof("Setting Unit(%s) target state to %s", u.Name, state)
		cAPI.SetUnitTargetState(u.Name, string(state))
		triggered = append(triggered, u)
	}

	return triggered, nil
}

// waitForUnitStates polls each of the indicated units until each of their
// states is equal to that which the caller indicates, or until the
// polling operation times out. waitForUnitStates will retry forever, or
// up to maxAttempts times before timing out if maxAttempts is greater
// than zero. Returned is an error channel used to communicate when
// timeouts occur. The returned error channel will be closed after all
// polling operation is complete.
func waitForUnitStates(units []string, js job.JobState, maxAttempts int, out io.Writer) chan error {
	errchan := make(chan error)
	var wg sync.WaitGroup
	for _, name := range units {
		wg.Add(1)
		go checkUnitState(name, js, maxAttempts, out, &wg, errchan)
	}

	go func() {
		wg.Wait()
		close(errchan)
	}()

	return errchan
}

func checkUnitState(name string, js job.JobState, maxAttempts int, out io.Writer, wg *sync.WaitGroup, errchan chan error) {
	defer wg.Done()

	sleep := 100 * time.Millisecond

	if maxAttempts < 1 {
		for {
			if assertUnitState(name, js, out) {
				return
			}
			time.Sleep(sleep)
		}
	} else {
		for attempt := 0; attempt < maxAttempts; attempt++ {
			if assertUnitState(name, js, out) {
				return
			}
			time.Sleep(sleep)
		}
		errchan <- fmt.Errorf("timed out waiting for unit %s to report state %s", name, js)
	}
}

func assertUnitState(name string, js job.JobState, out io.Writer) (ret bool) {
	u, err := cAPI.Unit(name)
	if err != nil {
		log.Warningf("Error retrieving Unit(%s) from Registry: %v", name, err)
		return
	}
	if u == nil || job.JobState(u.CurrentState) != js {
		return
	}

	ret = true
	msg := fmt.Sprintf("Unit %s %s", name, u.CurrentState)

	if u.MachineID == "" {
		return
	}

	ms := cachedMachineState(u.MachineID)
	if ms != nil {
		msg = fmt.Sprintf("%s on %s", msg, machineFullLegend(*ms, false))
	}

	fmt.Fprintln(out, msg)
	return
}

func machineState(machID string) (*machine.MachineState, error) {
	machines, err := cAPI.Machines()
	if err != nil {
		return nil, err
	}
	for _, ms := range machines {
		if ms.ID == machID {
			return &ms, nil
		}
	}
	return nil, nil
}

// cachedMachineState makes a best-effort to retrieve the MachineState of the given machine ID.
// It memoizes MachineState information for the life of a fleetctl invocation.
// Any error encountered retrieving the list of machines is ignored.
func cachedMachineState(machID string) (ms *machine.MachineState) {
	if machineStates == nil {
		machineStates = make(map[string]*machine.MachineState)
		ms, err := cAPI.Machines()
		if err != nil {
			return nil
		}
		for i, m := range ms {
			machineStates[m.ID] = &ms[i]
		}
	}
	return machineStates[machID]
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

// suToGlobal returns whether or not a schema.Unit refers to a global unit
func suToGlobal(su schema.Unit) bool {
	u := job.Unit{
		Unit: *schema.MapSchemaUnitOptionsToUnitFile(su.Options),
	}
	return u.IsGlobal()
}
