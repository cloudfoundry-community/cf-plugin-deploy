package main

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/cli/plugin"
)

type Plugin struct{}

func (p Plugin) Run(c plugin.CliConnection, args []string) {
	m, err := ParseManifest(os.Stdin)
	if err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}

	d := &Deployer{
		manifest: &m,
		cf:       c,
	}
	if err := d.Deploy(); err != nil {
		fmt.Printf("ERROR: %s\n", err)
		os.Exit(1)
	}
}

func (Plugin) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "deploy",
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
