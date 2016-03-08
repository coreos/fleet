// Copyright 2014 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	etcd "github.com/coreos/fleet/Godeps/_workspace/src/github.com/coreos/etcd/client"

	"github.com/coreos/fleet/api"
	"github.com/coreos/fleet/client"
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

	clientDriverAPI  = "API"
	clientDriverEtcd = "etcd"

	defaultEndpoint  = "unix:///var/run/fleet.sock"
	defaultSleepTime = 500 * time.Millisecond
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
		Debug   bool
		Version bool
		Help    bool

		ClientDriver    string
		ExperimentalAPI bool
		Endpoint        string
		RequestTimeout  float64

		KeyFile  string
		CertFile string
		CAFile   string

		Tunnel                string
		KnownHostsFile        string
		StrictHostKeyChecking bool
		SSHTimeout            float64
		SSHUserName           string

		EtcdKeyPrefix string
	}{}

	// flags used by multiple commands
	sharedFlags = struct {
		Sign          bool
		Full          bool
		NoLegend      bool
		NoBlock       bool
		BlockAttempts int
		Fields        string
		SSHPort       int
	}{}

	// used to cache MachineStates
	machineStates map[string]*machine.MachineState
)

func init() {
	// call this as early as possible to ensure we always have timestamps
	// on fleetctl logs
	log.EnableTimestamps()

	globalFlagset.BoolVar(&globalFlags.Help, "help", false, "Print usage information and exit")
	globalFlagset.BoolVar(&globalFlags.Help, "h", false, "Print usage information and exit")

	globalFlagset.BoolVar(&globalFlags.Debug, "debug", false, "Print out more debug information to stderr")
	globalFlagset.BoolVar(&globalFlags.Version, "version", false, "Print the version and exit")
	globalFlagset.StringVar(&globalFlags.ClientDriver, "driver", clientDriverAPI, fmt.Sprintf("Adapter used to execute fleetctl commands. Options include %q and %q.", clientDriverAPI, clientDriverEtcd))
	globalFlagset.StringVar(&globalFlags.Endpoint, "endpoint", defaultEndpoint, fmt.Sprintf("Location of the fleet API if --driver=%s. Alternatively, if --driver=%s, location of the etcd API.", clientDriverAPI, clientDriverEtcd))
	globalFlagset.StringVar(&globalFlags.EtcdKeyPrefix, "etcd-key-prefix", registry.DefaultKeyPrefix, "Keyspace for fleet data in etcd (development use only!)")

	globalFlagset.StringVar(&globalFlags.KeyFile, "key-file", "", "Location of TLS key file used to secure communication with the fleet API or etcd")
	globalFlagset.StringVar(&globalFlags.CertFile, "cert-file", "", "Location of TLS cert file used to secure communication with the fleet API or etcd")
	globalFlagset.StringVar(&globalFlags.CAFile, "ca-file", "", "Location of TLS CA file used to secure communication with the fleet API or etcd")

	globalFlagset.StringVar(&globalFlags.KnownHostsFile, "known-hosts-file", ssh.DefaultKnownHostsFile, "File used to store remote machine fingerprints. Ignored if strict host key checking is disabled.")
	globalFlagset.BoolVar(&globalFlags.StrictHostKeyChecking, "strict-host-key-checking", true, "Verify host keys presented by remote machines before initiating SSH connections.")
	globalFlagset.Float64Var(&globalFlags.SSHTimeout, "ssh-timeout", 10.0, "Amount of time in seconds to allow for SSH connection initialization before failing.")
	globalFlagset.StringVar(&globalFlags.Tunnel, "tunnel", "", "Establish an SSH tunnel through the provided address for communication with fleet and etcd.")
	globalFlagset.Float64Var(&globalFlags.RequestTimeout, "request-timeout", 3.0, "Amount of time in seconds to allow a single request before considering it failed.")
	globalFlagset.StringVar(&globalFlags.SSHUserName, "ssh-username", "core", "Username to use when connecting to CoreOS instance.")

	// deprecated flags
	globalFlagset.BoolVar(&globalFlags.ExperimentalAPI, "experimental-api", true, hidden)
	globalFlagset.StringVar(&globalFlags.KeyFile, "etcd-keyfile", "", hidden)
	globalFlagset.StringVar(&globalFlags.CertFile, "etcd-certfile", "", hidden)
	globalFlagset.StringVar(&globalFlags.CAFile, "etcd-cafile", "", hidden)
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
		cmdFDForward,
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

func maybeAddNewline(s string) string {
	if !strings.HasSuffix(s, "\n") {
		s = s + "\n"
	}
	return s
}

func stderr(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, maybeAddNewline(format), args...)
}

func stdout(format string, args ...interface{}) {
	fmt.Fprintf(os.Stdout, maybeAddNewline(format), args...)
}

// checkVersion makes a best-effort attempt to verify that fleetctl is at least as new as the
// latest fleet version found registered in the cluster. If any errors are encountered or fleetctl
// is >= the latest version found, it returns true. If it is < the latest found version, it returns
// false and a scary warning to the user.
func checkVersion(cReg registry.ClusterRegistry) (string, bool) {
	fv := version.SemVersion
	lv, err := cReg.LatestDaemonVersion()
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
		log.EnableDebug()
	}

	if globalFlags.Version {
		args = []string{"version"}
	} else if len(args) < 1 || globalFlags.Help {
		args = []string{"help"}
	}

	var cmd *Command

	// determine which Command should be run
	for _, c := range commands {
		if c.Name == args[0] {
			cmd = c
			if err := c.Flags.Parse(args[1:]); err != nil {
				stderr("%v", err)
				os.Exit(2)
			}
			break
		}
	}

	if cmd == nil {
		stderr("%v: unknown subcommand: %q", cliName, args[0])
		stderr("Run '%v help' for usage.", cliName)
		os.Exit(2)
	}

	if sharedFlags.Sign {
		stderr("WARNING: The signed/verified units feature is DEPRECATED and cannot be used.")
		os.Exit(2)
	}

	visited := make(map[string]bool, 0)
	globalFlagset.Visit(func(f *flag.Flag) { visited[f.Name] = true })

	// if --driver is not set, but --endpoint looks like an etcd
	// server, set the driver to etcd
	if visited["endpoint"] && !visited["driver"] {
		if u, err := url.Parse(strings.Split(globalFlags.Endpoint, ",")[0]); err == nil {
			if _, port, err := net.SplitHostPort(u.Host); err == nil && (port == "4001" || port == "2379") {
				log.Debugf("Defaulting to --driver=%s as --endpoint appears to be etcd", clientDriverEtcd)
				globalFlags.ClientDriver = clientDriverEtcd
			}
		}
	}

	if cmd.Name != "help" && cmd.Name != "version" {
		var err error
		cAPI, err = getClient()
		if err != nil {
			stderr("Unable to initialize client: %v", err)
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
	// The user explicitly set --experimental-api=false, so it trumps the
	// --driver flag. This behavior exists for backwards-compatibilty.
	if !globalFlags.ExperimentalAPI {
		// Additionally, if the user set --experimental-api=false and did
		// not change the value of --endpoint, they likely want to use the
		// old default value.
		if globalFlags.Endpoint == defaultEndpoint {
			globalFlags.Endpoint = "http://127.0.0.1:2379,http://127.0.0.1:4001"
		}
		return getRegistryClient()
	}

	switch globalFlags.ClientDriver {
	case clientDriverAPI:
		return getHTTPClient()
	case clientDriverEtcd:
		return getRegistryClient()
	}

	return nil, fmt.Errorf("unrecognized driver %q", globalFlags.ClientDriver)
}

func getHTTPClient() (client.API, error) {
	endpoints := strings.Split(globalFlags.Endpoint, ",")
	if len(endpoints) > 1 {
		log.Warningf("multiple endpoints provided but only the first (%s) is used", endpoints[0])
	}

	ep, err := url.Parse(endpoints[0])
	if err != nil {
		return nil, err
	}

	if len(ep.Scheme) == 0 {
		return nil, errors.New("URL scheme undefined")
	}

	tun := getTunnelFlag()
	tunneling := tun != ""

	dialUnix := ep.Scheme == "unix" || ep.Scheme == "file"

	tunnelFunc := net.Dial
	if tunneling {
		sshClient, err := ssh.NewSSHClient(globalFlags.SSHUserName, tun, getChecker(), true, getSSHTimeoutFlag())
		if err != nil {
			return nil, fmt.Errorf("failed initializing SSH client: %v", err)
		}

		if dialUnix {
			tgt := ep.Path
			tunnelFunc = func(string, string) (net.Conn, error) {
				log.Debugf("Establishing remote fleetctl proxy to %s", tgt)
				cmd := fmt.Sprintf(`fleetctl fd-forward %s`, tgt)
				return ssh.DialCommand(sshClient, cmd)
			}
		} else {
			tunnelFunc = sshClient.Dial
		}
	}

	dialFunc := tunnelFunc
	if dialUnix {
		// This commonly happens if the user misses the leading slash after the scheme.
		// For example, "unix://var/run/fleet.sock" would be parsed as host "var".
		if len(ep.Host) > 0 {
			return nil, fmt.Errorf("unable to connect to host %q with scheme %q", ep.Host, ep.Scheme)
		}

		// The Path field is only used for dialing and should not be used when
		// building any further HTTP requests.
		sockPath := ep.Path
		ep.Path = ""

		// If not tunneling to the unix socket, http.Client will dial it directly.
		// http.Client does not natively support dialing a unix domain socket, so the
		// dial function must be overridden.
		if !tunneling {
			dialFunc = func(string, string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			}
		}

		// http.Client doesn't support the schemes "unix" or "file", but it
		// is safe to use "http" as dialFunc ignores it anyway.
		ep.Scheme = "http"

		// The Host field is not used for dialing, but will be exposed in debug logs.
		ep.Host = "domain-sock"
	}

	tlsConfig, err := pkg.ReadTLSConfigFiles(globalFlags.CAFile, globalFlags.CertFile, globalFlags.KeyFile)
	if err != nil {
		return nil, err
	}

	trans := pkg.LoggingHTTPTransport{
		Transport: http.Transport{
			Dial:            dialFunc,
			TLSClientConfig: tlsConfig,
		},
	}

	hc := http.Client{
		Transport: &trans,
	}

	return client.NewHTTPClient(&hc, *ep)
}

func getRegistryClient() (client.API, error) {
	var dial func(string, string) (net.Conn, error)
	tun := getTunnelFlag()
	if tun != "" {
		sshClient, err := ssh.NewSSHClient(globalFlags.SSHUserName, tun, getChecker(), false, getSSHTimeoutFlag())
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

	tlsConfig, err := pkg.ReadTLSConfigFiles(globalFlags.CAFile, globalFlags.CertFile, globalFlags.KeyFile)
	if err != nil {
		return nil, err
	}

	trans := &http.Transport{
		Dial:            dial,
		TLSClientConfig: tlsConfig,
	}

	eCfg := etcd.Config{
		Endpoints: strings.Split(globalFlags.Endpoint, ","),
		Transport: trans,
	}

	eClient, err := etcd.New(eCfg)
	if err != nil {
		return nil, err
	}

	kAPI := etcd.NewKeysAPI(eClient)
	reg := registry.NewEtcdRegistry(kAPI, globalFlags.EtcdKeyPrefix, getRequestTimeoutFlag())

	if msg, ok := checkVersion(reg); !ok {
		stderr(msg)
	}

	return &client.RegistryClient{Registry: reg}, nil
}

// getChecker creates and returns a HostKeyChecker, or nil if any error is encountered
func getChecker() *ssh.HostKeyChecker {
	if !globalFlags.StrictHostKeyChecking {
		return nil
	}

	keyFile := ssh.NewHostKeyFile(globalFlags.KnownHostsFile)
	return ssh.NewHostKeyChecker(keyFile)
}

// getUnitFile attempts to get a UnitFile configuration
// It takes a unit file name as a parameter and tries first to lookup
// the unit from the local disk. If it fails, it checks if the provided
// file name may reference an instance of a template unit, if so, it
// tries to get the template configuration either from the registry or
// the local disk.
// It returns a UnitFile configuration or nil; and any error ecountered
func getUnitFile(file string) (*unit.UnitFile, error) {
	var uf *unit.UnitFile
	name := unitNameMangle(file)

	log.Debugf("Looking for Unit(%s) or its corresponding template", name)

	// Assume that the file references a local unit file on disk and
	// attempt to load it, if it exists
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		uf, err = getUnitFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed getting Unit(%s) from file: %v", file, err)
		}
	} else {
		// Otherwise (if the unit file does not exist), check if the
		// name appears to be an instance of a template unit
		info := unit.NewUnitNameInfo(name)
		if info == nil {
			return nil, fmt.Errorf("error extracting information from unit name %s", name)
		} else if !info.IsInstance() {
			return nil, fmt.Errorf("unable to find Unit(%s) in Registry or on filesystem", name)
		}

		// If it is an instance check for a corresponding template
		// unit in the Registry or disk.
		// If we found a template unit, later we create a
		// near-identical instance unit in the Registry - same
		// unit file as the template, but different name
		uf, err = getUnitFileFromTemplate(info, file)
		if err != nil {
			return nil, fmt.Errorf("failed getting Unit(%s) from template: %v", file, err)
		}
	}

	log.Debugf("Found Unit(%s)", name)
	return uf, nil
}

// getUnitFromFile attempts to load a Unit from a given filename
// It returns the Unit or nil, and any error encountered
func getUnitFromFile(file string) (*unit.UnitFile, error) {
	out, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	unitName := path.Base(file)
	log.Debugf("Unit(%s) found in local filesystem", unitName)

	return unit.NewUnitFile(string(out))
}

// getUnitFileFromTemplate attempts to get a Unit from a template unit that
// is either in the registry or on the file system
// It takes two arguments, the template information and the unit file name
// It returns the Unit or nil; and any error encountered
func getUnitFileFromTemplate(uni *unit.UnitNameInfo, fileName string) (*unit.UnitFile, error) {
	var uf *unit.UnitFile

	tmpl, err := cAPI.Unit(uni.Template)
	if err != nil {
		return nil, fmt.Errorf("error retrieving template Unit(%s) from Registry: %v", uni.Template, err)
	}

	if tmpl != nil {
		warnOnDifferentLocalUnit(fileName, tmpl)
		uf = schema.MapSchemaUnitOptionsToUnitFile(tmpl.Options)
		log.Debugf("Template Unit(%s) found in registry", uni.Template)
	} else {
		// Finally, if we could not find a template unit in the Registry,
		// check the local disk for one instead
		filePath := path.Join(path.Dir(fileName), uni.Template)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("unable to find template Unit(%s) in Registry or on filesystem", uni.Template)
		}

		uf, err = getUnitFromFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("unable to load template Unit(%s) from file: %v", uni.Template, err)
		}
	}

	return uf, nil
}

func getTunnelFlag() string {
	tun := globalFlags.Tunnel
	if tun != "" && !strings.Contains(tun, ":") {
		tun += ":22"
	}
	return tun
}

func getSSHTimeoutFlag() time.Duration {
	return time.Duration(globalFlags.SSHTimeout*1000) * time.Millisecond
}

func getRequestTimeoutFlag() time.Duration {
	return time.Duration(globalFlags.RequestTimeout*1000) * time.Millisecond
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
	if uf == nil {
		return nil, fmt.Errorf("nil unit provided")
	}
	u := schema.Unit{
		Name:    name,
		Options: schema.MapUnitFileToSchemaUnitOptions(uf),
	}
	// TODO(jonboulle): this dependency on the API package is awkward, and
	// redundant with the check in api.unitsResource.set, but it is a
	// workaround to implementing the same check in the RegistryClient. It
	// will disappear once RegistryClient is deprecated.
	if err := api.ValidateName(name); err != nil {
		return nil, err
	}
	if err := api.ValidateOptions(u.Options); err != nil {
		return nil, err
	}
	j := &job.Job{Unit: *uf}
	if err := j.ValidateRequirements(); err != nil {
		log.Warningf("Unit %s: %v", name, err)
	}
	err := cAPI.CreateUnit(&u)
	if err != nil {
		return nil, fmt.Errorf("failed creating unit %s: %v", name, err)
	}

	log.Debugf("Created Unit(%s) in Registry", name)
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
	errchan := make(chan error)
	var wg sync.WaitGroup
	for _, arg := range args {
		arg = maybeAppendDefaultUnitType(arg)
		name := unitNameMangle(arg)

		// First, check if there already exists a Unit by the given name in the Registry
		u, err := cAPI.Unit(name)
		if err != nil {
			return fmt.Errorf("error retrieving Unit(%s) from Registry: %v", name, err)
		}
		if u != nil {
			log.Debugf("Found Unit(%s) in Registry, no need to recreate it", name)
			warnOnDifferentLocalUnit(arg, u)
			continue
		}

		// Assume that the name references a local unit file on
		// disk or if it is an instance unit and if so get its
		// corresponding unit
		uf, err := getUnitFile(arg)
		if err != nil {
			return err
		}

		_, err = createUnit(name, uf)
		if err != nil {
			return err
		}

		wg.Add(1)
		go checkUnitState(name, job.JobStateInactive, sharedFlags.BlockAttempts, os.Stdout, &wg, errchan)
	}

	go func() {
		wg.Wait()
		close(errchan)
	}()

	haserr := false
	for msg := range errchan {
		stderr("Error waiting on unit creation: %v", msg)
		haserr = true
	}

	if haserr {
		return fmt.Errorf("One or more errors creating units")
	}

	return nil
}

func warnOnDifferentLocalUnit(loc string, su *schema.Unit) {
	suf := schema.MapSchemaUnitOptionsToUnitFile(su.Options)
	if _, err := os.Stat(loc); !os.IsNotExist(err) {
		luf, err := getUnitFromFile(loc)
		if err == nil && luf.Hash() != suf.Hash() {
			stderr("WARNING: Unit %s in registry differs from local unit file %s", su.Name, loc)
			return
		}
	}
	if uni := unit.NewUnitNameInfo(path.Base(loc)); uni != nil && uni.IsInstance() {
		file := path.Join(path.Dir(loc), uni.Template)
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			tmpl, err := getUnitFromFile(file)
			if err == nil && tmpl.Hash() != suf.Hash() {
				stderr("WARNING: Unit %s in registry differs from local template unit file %s", su.Name, uni.Template)
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
			log.Debugf("Unit(%s) already %s, skipping.", u.Name, u.DesiredState)
			continue
		}

		log.Debugf("Setting Unit(%s) target state to %s", u.Name, state)
		if err := cAPI.SetUnitTargetState(u.Name, string(state)); err != nil {
			return nil, err
		}
		triggered = append(triggered, u)
	}

	return triggered, nil
}

// getBlockAttempts gets the correct value of how many attempts to try
// before giving up on an operation.
// It returns a negative value which means do not block, if zero is
// returned then it means try forever, and if a positive value is
// returned then try up to that value
func getBlockAttempts() int {
	// By default we wait forever
	var attempts int = 0

	if sharedFlags.BlockAttempts > 0 {
		attempts = sharedFlags.BlockAttempts
	}

	if sharedFlags.NoBlock {
		attempts = -1
	}

	return attempts
}

// tryWaitForUnitStates tries to wait for units to reach the desired state.
// It takes 5 arguments, the units to wait for, the desired state, the
// desired JobState, how many attempts before timing out and a writer
// interface.
// tryWaitForUnitStates polls each of the indicated units until they
// reach the desired state. If maxAttempts is negative, then it will not
// wait, it will assume that all units reached their desired state.
// If maxAttempts is zero tryWaitForUnitStates will retry forever, and
// if it is greater than zero, it will retry up to the indicated value.
// It returns 0 on success or 1 on errors.
func tryWaitForUnitStates(units []string, state string, js job.JobState, maxAttempts int, out io.Writer) (ret int) {
	// We do not wait just assume we reached the desired state
	if maxAttempts <= -1 {
		for _, name := range units {
			stdout("Triggered unit %s %s", name, state)
		}
		return
	}

	errchan := waitForUnitStates(units, js, maxAttempts, out)
	for err := range errchan {
		stderr("Error waiting for units: %v", err)
		ret = 1
	}

	return
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

	sleep := defaultSleepTime

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
	var state string

	u, err := cAPI.Unit(name)
	if err != nil {
		log.Warningf("Error retrieving Unit(%s) from Registry: %v", name, err)
		return
	}
	if u == nil {
		log.Warningf("Unit %s not found", name)
		return
	}

	// If this is a global unit, CurrentState will never be set. Instead, wait for DesiredState.
	if suToGlobal(*u) {
		state = u.DesiredState
	} else {
		state = u.CurrentState
	}

	if job.JobState(state) != js {
		log.Debugf("Waiting for Unit(%s) state(%s) to be %s", name, job.JobState(state), js)
		return
	}

	ret = true
	msg := fmt.Sprintf("Unit %s %s", name, u.CurrentState)

	if u.MachineID != "" {
		ms := cachedMachineState(u.MachineID)
		if ms != nil {
			msg = fmt.Sprintf("%s on %s", msg, machineFullLegend(*ms, false))
		}
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
func unitNameMangle(arg string) string {
	return maybeAppendDefaultUnitType(path.Base(arg))
}

func maybeAppendDefaultUnitType(arg string) string {
	if !unit.RecognizedUnitType(arg) {
		arg = unit.DefaultUnitType(arg)
	}
	return arg
}

// suToGlobal returns whether or not a schema.Unit refers to a global unit
func suToGlobal(su schema.Unit) bool {
	u := job.Unit{
		Unit: *schema.MapSchemaUnitOptionsToUnitFile(su.Options),
	}
	return u.IsGlobal()
}
