package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/cloudfoundry/cli/plugin"
)

func isMissing(e error) bool {
	return strings.Contains(e.Error(), "not found")
}

func parseService(s string) (string, string) {
	x := strings.SplitN(s, "/", 2)
	return x[0], x[1]
}

func boolify(s string) bool {
	s = strings.ToLower(s)
	return s == "yes" || s == "y" || s == "on" || s == "enabled"
}

type Deployer struct {
	manifest *Manifest
	cf       plugin.CliConnection
}

func (d *Deployer) run(args ...string) error {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf(">> %s\n", strings.Join(args, " "))
	}
	if os.Getenv("DRYRUN") != "" {
		return nil
	}
	_, err := d.cf.CliCommandWithoutTerminalOutput(args...)
	return err
}

func (d *Deployer) createUser(user string) error {
	for _, u := range d.manifest.Users {
		if u.Name == user {
			/* if we have a username and password, let's set them!
			   (note that this fails miserably if the user exists but has a different
				password.  oh well.) */
			d.run("create-user", u.Name, u.Password)
			return nil
		}
	}
	/* since we can't query cf to see if the user exists (yet), let's
	   just assume that they do exist, and not create them if there is
	   no entry in the top-level `users` map from the manifest, mmmkay? */
	return nil
}

func (d *Deployer) createSharedDomain(domain string) error {
	/* there is currently no good way to determine if we failed to create
	   the shared domain because it already existed, or if there was some
	   other failure (auth / perms / bad input / etc.)

	   so for now, we just ignore *all* the errors and pretend everything
	   is going to be just fine thank you very much. */

	d.run("create-shared-domain", domain)
	return nil
}

func (d *Deployer) createOrg(org string) error {
	o, _ := d.cf.GetOrg(org)
	if o.Guid != "" {
		return nil
	}

	if err := d.run("create-org", org); err != nil {
		return err
	}

	return nil
}

func (d *Deployer) createOrgDomain(org, domain string) error {
	o, err := d.cf.GetOrg(org)
	if err != nil {
		return err
	}

	for _, d := range o.Domains {
		if d.Name == domain {
			return nil
		}
	}

	if err := d.run("create-domain", org, domain); err != nil {
		return err
	}

	return nil
}

func (d *Deployer) grantOrgRole(org, user, role string) error {
	_, err := d.cf.GetOrg(org)
	if err != nil {
		return err
	}

	users, err := d.cf.GetOrgUsers(org)
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.Username == user {
			for _, r := range u.Roles {
				if r == role {
					return nil
				}
			}
		}
	}

	return d.run("set-org-role", user, org, role)
}

func (d *Deployer) createSpace(org, space string) error {
	o, err := d.cf.GetOrg(org)
	if err != nil {
		return err
	}

	for _, s := range o.Spaces {
		if s.Name == space {
			return nil
		}
	}

	if err := d.run("target", "-o", org); err != nil {
		return err
	}
	if err := d.run("create-space", space); err != nil {
		return err
	}

	return nil
}

func (d *Deployer) enableSSH(space string, on bool) error {
	if on {
		return d.run("allow-space-ssh", space)
	}
	return d.run("disallow-space-ssh", space)
}

func (d *Deployer) grantSpaceRole(org, space, user, role string) error {
	_, err := d.cf.GetOrg(org)
	if err != nil {
		return err
	}

	if err := d.run("target", "-o", org); err != nil {
		return err
	}
	_, err = d.cf.GetSpace(space)
	if err != nil {
		return err
	}

	users, err := d.cf.GetSpaceUsers(org, space)
	if err != nil {
		return err
	}

	for _, u := range users {
		if u.Username == user {
			for _, r := range u.Roles {
				if r == role {
					return nil
				}
			}
		}
	}

	return d.run("set-space-role", user, org, space, role)
}

func (d *Deployer) stageApp(app *Application) error {
	args := []string{"push", app.Name, "--no-start", "-i", fmt.Sprintf("%v", app.Instances)}

	if app.Hostname != "" {
		args = append(args, "-n", app.Hostname)
	}
	if app.Domain != "" {
		args = append(args, "-d", app.Domain)
	}
	if app.Disk != "" {
		args = append(args, "-k", app.Disk)
	}
	if app.Memory != "" {
		args = append(args, "-m", app.Memory)
	}
	if app.Buildpack != "" {
		args = append(args, "-b", app.Buildpack)
	}
	if app.Image != "" {
		args = append(args, "-o", app.Image)
	} else if app.Repository != "" {
		wd, _ := os.Getwd()
		path := wd + "/apps/" + app.Name
		os.MkdirAll(path, 0777)

		files, _ := ioutil.ReadDir(path)

		if len(files) == 0 {
			gitPath, err := exec.LookPath("git")
			if err != nil {
				return err
			}
			if err := exec.Command(gitPath, "clone", app.Repository, path).Run(); err != nil {
				return err
			}
		}
		args = append(args, "-p", path)
	} else if app.Path != "" {
		args = append(args, "-p", app.Path)
	} else {
		return fmt.Errorf("No image, repository or path supplied for '%s' app", app.Name)
	}

	return d.run(args...)
}

func (d *Deployer) mapURLs(app *Application) error {
	a, err := d.cf.GetApp(app.Name)
	if err != nil {
		return err
	}

	want := map[string]URL{}
	for _, s := range app.URLs {
		url := ParseURL(s, app.Domain)
		want[url.String()] = url
	}

	have := map[string]URL{}
	for _, r := range a.Routes {
		url := URL{
			Host:   r.Host,
			Domain: r.Domain.Name,
		}

		if _, ok := want[url.String()]; ok {
			delete(want, url.String())
		} else {
			have[url.String()] = url
		}
	}

	for u, url := range have {
		fmt.Printf("    unmapping route %s\n", u)
		if err := d.run("unmap-route", app.Name, url.Domain, "--hostname", url.Host); err != nil {
			return err
		}
	}
	for u, url := range want {
		fmt.Printf("    mapping route %s\n", u)
		if err := d.run("map-route", app.Name, url.Domain, "--hostname", url.Host); err != nil {
			return err
		}
	}

	return nil
}

func (d *Deployer) setEnvVar(name, value, app string) error {
	return d.run("set-env", app, name, value)
}

func (d *Deployer) startApp(app *Application) error {
	return d.run("start", app.Name)
}

func (d *Deployer) createService(name, broker, plan string) error {
	s, err := d.cf.GetServices()
	if err != nil {
		return err
	}

	for _, svc := range s {
		if svc.Name == name {
			/* FIXME: check configuration */
			return nil
		}
	}

	return d.run("create-service", broker, plan, name)
}

func (d *Deployer) bindService(service, app string) error {
	return d.run("bind-service", app, service)
}

func (d *Deployer) setQuotaArgs(quota *Quota) []string {
	var args []string
	if quota.Memory["total"] != "" {
		args = append(args, "-m", quota.Memory["total"])
	}
	if quota.Memory["per-app-instance"] != "" {
		perAppInstance := quota.Memory["per-app-instance"]
		if perAppInstance == "unlimited" {
			perAppInstance = "-1"
		}
		args = append(args, "-i", perAppInstance)
	}
	if quota.TotalAppInstances != "" {
		appInstances := quota.TotalAppInstances
		if appInstances == "unlimited" {
			appInstances = "-1"
		}
		args = append(args, "-a", quota.TotalAppInstances)
	}
	if quota.ServiceInstances != "" {
		args = append(args, "-s", quota.ServiceInstances)
	}
	if quota.Routes != "" {
		args = append(args, "-r", quota.Routes)
	}
	if quota.PaidPlans {
		args = append(args, "--allow-paid-service-plans")
	}
	if quota.NumRoutesWithResPorts != "" {
		args = append(args, "--reserved-route-ports", quota.NumRoutesWithResPorts)
	}
	return args
}

func (d *Deployer) createUpdateSpaceQuota(qname string, quota *Quota, oname string) error {
	org, _ := d.cf.GetOrg(oname)
	if org.Guid == "" {
		return nil
	}
	if err := d.run("target", "-o", oname); err != nil {
		return err
	}
	args := []string{"create-space-quota", qname}
	args = append(args, d.setQuotaArgs(quota)...)
	for _, cname := range org.SpaceQuotas {
		if cname.Name == qname {
			args[0] = "update-space-quota"
		}
	}

	return d.run(args...)
}

func (d *Deployer) createOrgQuota(qname string) error {
	return d.run("create-quota", qname)
}

func (d *Deployer) updateOrgQuota(qname string, quota *Quota) error {
	args := []string{"update-quota", qname}
	args = append(args, d.setQuotaArgs(quota)...)
	return d.run(args...)
}

func (d *Deployer) setQuota(name, quota string, space bool) error {
	cmd := "set-quota"
	if space {
		cmd = "set-space-quota"
	}
	return d.run(cmd, name, quota)
}

func (d *Deployer) Deploy() error {
	for _, domain := range d.manifest.Domains {
		fmt.Printf("setting up shared (global) domain '%s'\n", domain)
		if err := d.createSharedDomain(domain); err != nil {
			return err
		}
	}
	for qname, quota := range d.manifest.Quotas {
		fmt.Printf("creating/updating org quota '%s'\n", qname)
		// NOTE: create and update are separated because there is currently no way
		//       to pull existing top-level quota information out. This method
		//       avoids errors/failures.
		if err := d.createOrgQuota(qname); err != nil {
			return err
		}
		if err := d.updateOrgQuota(qname, quota); err != nil {
			return err
		}
	}

	for oname, org := range d.manifest.Organizations {
		fmt.Printf("creating organization '%s'\n", oname)
		if err := d.createOrg(oname); err != nil {
			return err
		}

		for _, domain := range org.Domains {
			fmt.Printf("  setting up organization domain '%s'\n", domain)
			if err := d.createOrgDomain(oname, domain); err != nil {
				return err
			}
		}
		if org.Quota != "" {
			fmt.Printf("  applying organization quota '%s'\n", org.Quota)
			if err := d.setQuota(oname, org.Quota, false); err != nil {
				return err
			}
		}
		for sqname, squota := range org.Quotas {
			fmt.Printf("  creating/updating space quota '%s'\n", sqname)
			if err := d.createUpdateSpaceQuota(sqname, squota, oname); err != nil {
				return err
			}
		}
		for uname, roles := range org.Users {
			fmt.Printf("  granting org-level access to user '%s'\n", uname)
			if err := d.createUser(uname); err != nil {
				return err
			}
			for _, role := range roles {
				fmt.Printf("    granting role '%s' to %s\n", role, uname)
				if err := d.grantOrgRole(oname, uname, role); err != nil {
					return err
				}
			}
		}
		for sname, space := range org.Spaces {
			fmt.Printf("  creating space '%s'\n", sname)
			if err := d.createSpace(oname, sname); err != nil {
				return err
			}
			if err := d.run("target", "-o", oname, "-s", sname); err != nil {
				return err
			}

			if space.SSH != "" {
				fmt.Printf("    setting ssh-enabled to '%s'\n", space.SSH)
				if err := d.enableSSH(sname, boolify(space.SSH)); err != nil {
					return err
				}
			}

			if space.Domain != "" {
				fmt.Printf("    using default domain of '%s'\n", space.Domain)
			}

			if space.Quota != "" {
				fmt.Printf("    applying space quota '%s'\n", space.Quota)
				if err := d.setQuota(sname, space.Quota, true); err != nil {
					return err
				}
			}

			for uname, roles := range space.Users {
				fmt.Printf("    granting space-level access to user '%s'\n", uname)
				if err := d.createUser(uname); err != nil {
					return err
				}
				for _, role := range roles {
					fmt.Printf("      granting role '%s' to %s\n", role, uname)
					if err := d.grantSpaceRole(oname, sname, uname, role); err != nil {
						return err
					}
				}
			}
			for svname, service := range space.SharedServices {
				fmt.Printf("    setting up shared service instance '%s' (from %s)\n", svname, service)
				broker, plan := parseService(service)
				if err := d.createService(svname, broker, plan); err != nil {
					return err
				}
			}
			for _, app := range space.Applications {
				fmt.Printf("    staging application '%s'\n", app.Name)
				fmt.Printf("      spinning up %d instances\n", app.Instances)
				if app.Hostname != "" {
					fmt.Printf("      using hostname '%s'\n", app.Hostname)
				}
				if app.Domain != "" {
					fmt.Printf("      using domain '%s'\n", app.Domain)
				}
				if app.Disk != "" {
					fmt.Printf("      provisioning with %s disk\n", app.Disk)
				}
				if app.Memory != "" {
					fmt.Printf("      provisioning with %s memory\n", app.Memory)
				}
				if app.Image != "" {
					fmt.Printf("      deploying image '%s'\n", app.Image)
				} else if app.Repository != "" {
					fmt.Printf("      deploying remote codebase from '%s'\n", app.Repository)
				} else if app.Path != "" {
					fmt.Printf("      deploying local codebase from '%s'\n", app.Path)
				}
				if app.Buildpack != "" {
					fmt.Printf("      using the '%s' buildpack\n", app.Buildpack)
				}

				if err := d.stageApp(app); err != nil {
					return err
				}

				if len(app.URLs) > 0 {
					if err := d.mapURLs(app); err != nil {
						return err
					}
				}

				for ename, value := range app.Environment {
					fmt.Printf("      setting environment variable $%s\n", ename)
					if err := d.setEnvVar(ename, value, app.Name); err != nil {
						return err
					}
				}

				for svname, service := range app.BoundServices {
					fmt.Printf("      binding service instance '%s' (from %s)\n", svname, service)
					broker, plan := parseService(service)
					if err := d.createService(svname, broker, plan); err != nil {
						return err
					}
					if err := d.bindService(svname, app.Name); err != nil {
						return err
					}
				}

				fmt.Printf("    starting application '%s'\n", app.Name)
				if err := d.startApp(app); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
