package packages

import "os/exec"

// KnownManager describes a well-known package manager with default command templates.
type KnownManager struct {
	Name        string
	Description string
	Def         PackageManagerDef
}

// Catalog is the list of well-known package managers, in display order.
var Catalog = []KnownManager{
	{
		Name:        "dnf",
		Description: "Fedora / RHEL / CentOS",
		Def: PackageManagerDef{
			Install:   "sudo dnf install -y {package}",
			Uninstall: "sudo dnf remove -y {package}",
			Update:    "sudo dnf upgrade -y {package}",
		},
	},
	{
		Name:        "yum",
		Description: "older RHEL / CentOS",
		Def: PackageManagerDef{
			Install:   "sudo yum install -y {package}",
			Uninstall: "sudo yum remove -y {package}",
			Update:    "sudo yum update -y {package}",
		},
	},
	{
		Name:        "apt",
		Description: "Debian / Ubuntu",
		Def: PackageManagerDef{
			Install:   "sudo apt-get install -y {package}",
			Uninstall: "sudo apt-get remove -y {package}",
			Update:    "sudo apt-get install -y {package}",
		},
	},
	{
		Name:        "pacman",
		Description: "Arch Linux",
		Def: PackageManagerDef{
			Install:   "sudo pacman -S --noconfirm {package}",
			Uninstall: "sudo pacman -R --noconfirm {package}",
			Update:    "sudo pacman -S --noconfirm {package}",
		},
	},
	{
		Name:        "brew",
		Description: "macOS / Linux (Homebrew)",
		Def: PackageManagerDef{
			Install:   "brew install {package}",
			Uninstall: "brew uninstall {package}",
			Update:    "brew upgrade {package}",
		},
	},
	{
		Name:        "zypper",
		Description: "openSUSE",
		Def: PackageManagerDef{
			Install:   "sudo zypper install -y {package}",
			Uninstall: "sudo zypper remove -y {package}",
			Update:    "sudo zypper update -y {package}",
		},
	},
	{
		Name:        "apk",
		Description: "Alpine Linux",
		Def: PackageManagerDef{
			Install:   "sudo apk add {package}",
			Uninstall: "sudo apk del {package}",
			Update:    "sudo apk upgrade {package}",
		},
	},
	{
		Name:        "cargo",
		Description: "Rust (crates.io)",
		Def: PackageManagerDef{
			Install:   "cargo install {package}",
			Uninstall: "cargo uninstall {package}",
			Update:    "cargo install {package}",
		},
	},
	{
		Name:        "npm",
		Description: "Node.js (global)",
		Def: PackageManagerDef{
			Install:   "npm install -g {package}",
			Uninstall: "npm uninstall -g {package}",
			Update:    "npm update -g {package}",
		},
	},
	{
		Name:        "pip3",
		Description: "Python 3",
		Def: PackageManagerDef{
			Install:   "pip3 install {package}",
			Uninstall: "pip3 uninstall -y {package}",
			Update:    "pip3 install --upgrade {package}",
		},
	},
}

// DetectInstalled returns the names of catalog managers present on PATH.
func DetectInstalled() []string {
	var found []string
	for _, m := range Catalog {
		if _, err := exec.LookPath(m.Name); err == nil {
			found = append(found, m.Name)
		}
	}
	return found
}
