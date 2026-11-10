package main

import (
	"flag"
	"fmt"
	"omasword/internal/cache"
	"omasword/internal/ui"
	"omasword/internal/version"
	"os"

	tea "charm.land/bubbletea/v2"
)

func main() {
	// Parse command line flags
	versionFlag := flag.Bool("version", false, "Print version information")
	flag.Parse()

	// Handle version flag
	if *versionFlag {
		fmt.Printf("omasword %s (build %s)\n", version.Version, version.BuildNumber)
		os.Exit(0)
	}
	// Initialize cache
	cacheManager, err := cache.NewCache()
	if err != nil {
		fmt.Printf("Warning: Could not initialize cache: %v\n", err)
		// Continue without cache
		cacheManager = nil
	}

	model := ui.NewModel()
	model.SetCache(cacheManager)

	p := tea.NewProgram(model)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
