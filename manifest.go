package main

import (
	"fmt"
	"io"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type User struct {
	Name     string `yaml:"username"`
	Password string `yaml:"password"`
}

type Organization struct {
	Users   map[string][]string `yaml:"users"`
	Domains []string            `yaml:"domains"`
	Spaces  map[string]Space    `yaml:"spaces"`
}

type Space struct {
	SSH            string              `yaml:"ssh"`
	Domain         string              `yaml:"domain"`
	Users          map[string][]string `yaml:"users"`
	SharedServices map[string]string   `yaml:"services"`
	Applications   []Application       `yaml:"apps"`
}

type Application struct {
	Name   string `yaml:"name"`
	Domain string `yaml:"domain"`

	Repository string `yaml:"repo"`
	Path       string `yaml:"path"`
	Image      string `yaml:"image"`

	Memory    string `yaml:"memory"`
	Disk      string `yaml:"disk"`
	Instances int    `yaml:"instances"`

	BoundServices  map[string]string `yaml:"bind"`
	SharedServices []string          `yaml:"shared"`
}

type Manifest struct {
	Domains       []string                `yaml:"domains"`
	Users         []User                  `yaml:"users"`
	Organizations map[string]Organization `yaml:"organizations"`
}

func ParseManifest(src io.Reader) (Manifest, error) {
	var m Manifest
	b, err := ioutil.ReadAll(src)
	if err != nil {
		return m, err
	}

	err = yaml.Unmarshal(b, &m)
	if err != nil {
		return m, err
	}

	/* resolve out the defaults */
	for o, org := range m.Organizations {
		for s, space := range org.Spaces {
			for a, app := range space.Applications {
				/* default to 1 instance of each application */
				if app.Instances < 1 {
					m.Organizations[o].Spaces[s].Applications[a].Instances = 1
				}

				/* use the default domain for the space, if present */
				if space.Domain != "" && app.Domain == "" {
					m.Organizations[o].Spaces[s].Applications[a].Domain = space.Domain
				}

				for _, svc := range app.SharedServices {
					bind, ok := space.SharedServices[svc]
					if !ok {
						return m, fmt.Errorf("reference to shared service '%s' in %s/%s application %s could not be found",
							svc, o, s, app.Name)
					}
					m.Organizations[o].Spaces[s].Applications[a].BoundServices[svc] = bind
				}
			}
		}
	}

	return m, nil
}
