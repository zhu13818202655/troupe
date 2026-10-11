package troupe

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/zhu13818202655/troupe/pkg/version"
)

func init() {
	// Register our version with pkg/version so the API can access it
	// Only register if version is not "dev" (i.e., was set via ldflags)
	// This allows both old CI (setting cmd/troupe.Version) and new CI (setting pkg/version.Version) to work
	if Version != "dev" {
		version.SetCmdVersion(Version)
	}
}

// Version is set at build time via -ldflags (for backward compat with CI)
var Version = "dev"

// Version information
const (
	Name   = "Troupe AI Companion"
	Author = "Rafael Schardosin Silva"
	GitHub = "https://github.com/zhu13818202655/troupe"
)

// ASCII Logo with colors using lipgloss
var asciiLogo = `
 _____
|_   _| _ __   ___   _   _  _ __    ___
  | |  | '__| / _ \ | | | || '_ \  / _ \
  | |  | |   | (_) || | | || |_) ||  __/
  | |  |_|    \___/  \__,_|| .__/  \___|
  \_/                      |_|
`

func printVersion() {
	// Styles
	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")). // Pink/Magenta
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("63")). // Purple
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")) // White/Grey

	linkStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")). // Blue
		Underline(true)

	// Print logo
	fmt.Println(logoStyle.Render(asciiLogo))
	fmt.Println()

	// Print version info
	fmt.Println(labelStyle.Render(Name))
	fmt.Printf("%s %s\n", labelStyle.Render("Version:"), valueStyle.Render(version.GetVersion()))
	fmt.Printf("%s %s\n", labelStyle.Render("Author:"), valueStyle.Render(Author))
	fmt.Printf("%s %s\n", labelStyle.Render("GitHub:"), linkStyle.Render(GitHub))
	fmt.Println()
}
