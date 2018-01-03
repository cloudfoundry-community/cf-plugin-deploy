package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

type URL struct {
	Host   string
	Domain string
}

func ParseURL(s, domain string) URL {
	p := strings.SplitN(s, ".", 2)
	if len(p) == 1 {
		p = append(p, domain)
	}
	return URL{
		Host:   p[0],
		Domain: p[1],
	}
}

func (u URL) String() string {
	if u.Domain == "" {
		return u.Host
	}
	return fmt.Sprintf("%s.%s", u.Host, u.Domain)
}

type User struct {
	Name     string `yaml:"username"`
	Password string `yaml:"password"`
}

type Organization struct {
	Users             map[string][]string `yaml:"users"`
	Domains           []string            `yaml:"domains"`
	Environment       map[string]string   `yaml:"env"`
	Spaces            map[string]*Space   `yaml:"spaces"`
	Quota             string              `yaml:"quota"`
	Quotas            map[string]*Quota   `yaml:"quotas"`
	SecurityGroupSets *SecurityGroupSet   `yaml:"security_group_sets"`
}

type Space struct {
	SSH                  string                 `yaml:"ssh"`
	Domain               string                 `yaml:"domain"`
	Users                map[string][]string    `yaml:"users"`
	Environment          map[string]string      `yaml:"env"`
	SharedServices       map[string]string      `yaml:"services"`
	Quota                string                 `yaml:"quota"`
	Applications         []*Application         `yaml:"apps"`
	UserProvidedServices []*UserProvidedService `yaml:"user-provided-services"`
	SecurityGroupSets    *SecurityGroupSet      `yaml:"security_group_sets"`
}

type Application struct {
	Name     string   `yaml:"name"`
	Hostname string   `yaml:"hostname"`
	Domain   string   `yaml:"domain"`
	URLs     []string `yaml:"urls"`

	Repository string `yaml:"repo"`
	Path       string `yaml:"path"`
	Image      string `yaml:"image"`
	Buildpack  string `yaml:"buildpack"`

	Memory      string            `yaml:"memory"`
	Disk        string            `yaml:"disk"`
	Instances   int               `yaml:"instances"`
	Environment map[string]string `yaml:"env"`

	BoundServices  map[string]string `yaml:"bind"`
	SharedServices []string          `yaml:"shared"`
}

type Quota struct {
	Memory                map[string]string `yaml:"memory"`
	TotalAppInstances     string            `yaml:"app-instances"`
	ServiceInstances      string            `yaml:"service-instances"`
	Routes                string            `yaml:"routes"`
	PaidPlans             bool              `yaml:"allow-paid-plans"`
	NumRoutesWithResPorts string            `yaml:"reserve-route-ports"`
}

type Manifest struct {
	Domains           []string                  `yaml:"domains"`
	Users             []User                    `yaml:"users"`
	Quotas            map[string]*Quota         `yaml:"quotas"`
	Organizations     map[string]*Organization  `yaml:"organizations"`
	SecurityGroups    map[string]*SecurityGroup `yaml:"security_groups"`
	SecurityGroupSets *SecurityGroupSet         `yaml:"security_group_sets"`
}

type UserProvidedService struct {
	Name            string      `yaml:"name"`
	Credentials     interface{} `yaml:"credentials"`
	RouteServiceUrl string      `yaml:"route_service_url"`
	SyslogDrainUrl  string      `yaml:"syslog_drain_url"`
}

type SecurityGroup struct {
	Rules             []interface{} `yaml:"rules"`
	SecurityGroupFile string        `yaml:"security_group_file"`
}

type SecurityGroupSet struct {
	Running []string `yaml:"running"`
	Staging []string `yaml:"staging"`
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
			shared := map[string]string{}
			for svc, details := range space.SharedServices {
				shared[fmt.Sprintf("%s-%s", "shared", svc)] = details
			}
			space.SharedServices = shared

			for a, app := range space.Applications {
				/* default to 1 instance of each application */
				if app.Instances < 1 {
					m.Organizations[o].Spaces[s].Applications[a].Instances = 1
				}

				/* if we have a hostname or domain, *and* URLs,
				   we need to throw an error. */
				if (app.Domain != "" || app.Hostname != "") && len(app.URLs) > 0 {
					return m, fmt.Errorf("Both hostname/domain and list of urls specified -- this is not allowed")
				}

				/* use the default domain for the space, if present */
				if space.Domain != "" && app.Domain == "" {
					m.Organizations[o].Spaces[s].Applications[a].Domain = space.Domain
				}

				services := map[string]string{}
				for svc, details := range app.BoundServices {
					services[fmt.Sprintf("%s-%s", app.Name, svc)] = details
				}
				for _, sv_ := range app.SharedServices {
					svc := fmt.Sprintf("shared-%s", sv_)
					bind, ok := space.SharedServices[svc]
					if !ok {
						return m, fmt.Errorf("reference to shared service '%s' in %s/%s application %s could not be found",
							svc, o, s, app.Name)
					}
					services[svc] = bind
				}
				app.BoundServices = services

				env := map[string]string{}
				for k, v := range org.Environment {
					env[k] = v
				}
				for k, v := range space.Environment {
					env[k] = v
				}
				for k, v := range app.Environment {
					env[k] = v
				}
				app.Environment = env
			}
		}
	}

	return m, nil
}
