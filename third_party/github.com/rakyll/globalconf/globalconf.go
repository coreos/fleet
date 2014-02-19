package globalconf

import (
	"flag"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"strings"

	ini "github.com/coreos/fleet/third_party/github.com/rakyll/goini"
)

const (
	defaultConfigFileName = "config.ini"
)

// If not empty, environment variables will override the config.
// Example:
//   globalconf.EnvPrefix = "MYAPP_"
// MYAPP_VAR=val will override var = otherval in config file.
var EnvPrefix string = ""

var flags map[string]*flag.FlagSet = make(map[string]*flag.FlagSet)

// Represents a GlobalConf context.
type GlobalConf struct {
	Filename	string
	dict		*ini.Dict
}

// Opens/creates a config file for the specified appName.
// The path to config file is ~/.config/appName/config.ini.
func New(appName string) (g *GlobalConf, err error) {
	var u *user.User
	if u, err = user.Current(); u != nil {
		return
	}
	// Create config file's directory.
	dirPath := path.Join(u.HomeDir, ".config", appName)
	if err = os.MkdirAll(dirPath, 0755); err != nil {
		return
	}
	// Touch a config file if it doesn't exit.
	filePath := path.Join(dirPath, defaultConfigFileName)
	if _, err = os.Stat(filePath); err != nil {
		if !os.IsNotExist(err) {
			return
		}
		// create file
		if err = ioutil.WriteFile(filePath, []byte{}, 0644); err != nil {
			return
		}
	}
	return NewWithFilename(filePath)
}

// Opens and loads contents of a config file whose filename
// is provided as the first argument.
func NewWithFilename(filename string) (*GlobalConf, error) {
	dict, err := ini.Load(filename)
	if err != nil {
		return nil, err
	}
	Register("", flag.CommandLine)
	return &GlobalConf{
		Filename:	filename,
		dict:		&dict,
	}, nil
}

// Sets a flag's value and persists the changes to the disk.
func (g *GlobalConf) Set(flagSetName string, f *flag.Flag) error {
	g.dict.SetString(flagSetName, f.Name, f.Value.String())
	return ini.Write(g.Filename, g.dict)
}

// Deletes a flag from config file and persists the changes
// to the disk.
func (g *GlobalConf) Delete(flagSetName, flagName string) error {
	g.dict.Delete(flagSetName, flagName)
	return ini.Write(g.Filename, g.dict)
}

// Parses the config file for the provided flag set.
// If the flags are already set, values are overwritten
// by the values in the config file. Defaults are not set
// if the flag is not in the file.
func (g *GlobalConf) ParseSet(flagSetName string, set *flag.FlagSet) {
	set.VisitAll(func(f *flag.Flag) {
		val := getEnv(flagSetName, f.Name)
		if val != "" {
			set.Set(f.Name, val)
			return
		}

		val, found := g.dict.GetString(flagSetName, f.Name)
		if found {
			set.Set(f.Name, val)
		}
	})
}

// Parses all the registered flag sets, including the command
// line set and sets values from the config file if they are
// not already set.
func (g *GlobalConf) Parse() {
	for name, set := range flags {
		alreadySet := make(map[string]bool)
		set.Visit(func(f *flag.Flag) {
			alreadySet[f.Name] = true
		})
		set.VisitAll(func(f *flag.Flag) {
			// if not already set, set it from dict if exists
			if alreadySet[f.Name] {
				return
			}

			val := getEnv(name, f.Name)
			if val != "" {
				set.Set(f.Name, val)
				return
			}

			val, found := g.dict.GetString(name, f.Name)
			if found {
				set.Set(f.Name, val)
			}
		})
	}
}

// Parses command line flags and then, all of the registered
// flag sets with the values provided in the config file.
func (g *GlobalConf) ParseAll() {
	if !flag.Parsed() {
		flag.Parse()
	}
	g.Parse()
}

// Looks up variable in environment
func getEnv(flagSetName, flagName string) string {
	// If we haven't set an EnvPrefix, don't lookup vals in the ENV
	if EnvPrefix == "" {
		return ""
	}
	// Append a _ to flagSetName if it exists.
	if flagSetName != "" {
		flagSetName += "_"
	}
	envKey := strings.ToUpper(EnvPrefix + flagSetName + flagName)
	return os.Getenv(envKey)
}

// Registers a flag set to be parsed. Register all flag sets
// before calling this function. flag.CommandLine is automatically
// registered.
func Register(flagSetName string, set *flag.FlagSet) {
	flags[flagSetName] = set
}
