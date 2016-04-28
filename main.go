package main

import (
	"fmt"
	"os"
)

func main() {
	m, err := ParseManifest(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	for _, domain := range m.Domains {
		fmt.Printf("setting up global domain '%s'\n", domain)
	}
	for name, org := range m.Organizations {
		fmt.Printf("creating organization '%s'\n", name)
		for _, domain := range org.Domains {
			fmt.Printf("  setting up organization domain '%s'\n", domain)
		}
		for name, roles := range org.Users {
			fmt.Printf("  granting org-level access to user '%s'\n", name)
			for _, role := range roles {
				fmt.Printf("    granting role '%s' to %s\n", role, name)
			}
		}
		for name, space := range org.Spaces {
			fmt.Printf("  creating space '%s'\n", name)
			fmt.Printf("    setting ssh-enabled to '%s'\n", space.SSH)
			fmt.Printf("    using default domain of '%s'\n", space.Domain)
			for name, roles := range space.Users {
				fmt.Printf("    granting space-level access to user '%s'\n", name)
				for _, role := range roles {
					fmt.Printf("      granting role '%s' to %s\n", role, name)
				}
			}
			for name, service := range space.SharedServices {
				fmt.Printf("    setting up shared service instance '%s' (from %s)\n", name, service)
			}
			for _, app := range space.Applications {
				fmt.Printf("    deploying application '%s'\n", app.Name)
				fmt.Printf("      spinning up %d instances\n", app.Instances)
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

				for name, service := range app.BoundServices {
					fmt.Printf("      binding service instance '%s' (from %s)\n", name, service)
				}
			}
		}
	}
}
