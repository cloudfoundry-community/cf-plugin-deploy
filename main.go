package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cloudfoundry/cli/plugin"
)

type Plugin struct{}

func (p Plugin) Run(c plugin.CliConnection, args []string) {
	m, err := ParseManifest(os.Stdin)
	if err != nil {
		fmt.Printf("Failed to parse manifest from standard input: %s\n", err)
		os.Exit(1)
	}

	d := &Deployer{
		manifest: &m,
		cf:       c,
	}
	if err := d.Deploy(); err != nil {
		fmt.Printf("Deployment failed: %s\n", err)
		os.Exit(1)
	}
}

var Version string

func vnum(s string) ([]int, error) {
	n := strings.Split(Version, ".")
	ints := make([]int, len(n))
	for i, x := range n {
		num, err := strconv.Atoi(x)
		if err != nil {
			return ints, err
		}
		if num < 0 {
			return ints, fmt.Errorf("Invalid version component '%s'", x)
		}
		ints[i] = num
	}
	return ints, nil
}

func getVersion(s string) (v plugin.VersionType) {
	n, err := vnum(s)
	if err != nil {
		return
	}
	if len(n) >= 1 {
		v.Major = n[0]
	}
	if len(n) >= 2 {
		v.Minor = n[1]
	}
	if len(n) >= 3 {
		v.Build = n[2]
	}
	return
}

func (Plugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name:    "deploy",
		Version: getVersion(Version),
		Commands: []plugin.Command{
			{
				Name:     "deploy",
				HelpText: "Deploys all the things, including orgs, spaces, domains, users, services and applications",
			},
		},
	}
}

func main() {
	plugin.Start(&Plugin{})
}
