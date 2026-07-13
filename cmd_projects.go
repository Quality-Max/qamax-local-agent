package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func cmdProjects(args []string) {
	fs := flag.NewFlagSet("projects", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "Output raw JSON")
	_ = fs.Parse(args)

	cfg, err := LoadConfig()
	if err != nil || cfg.Token == "" {
		fmt.Fprintln(os.Stderr, "Error: not logged in. Run `qmax login` first.")
		os.Exit(1)
	}

	apiURL := cfg.GetAPIBaseURL()
	body := authGet(cfg, fmt.Sprintf("%s/api/projects", apiURL))

	if *jsonOut {
		fmt.Println(string(body))
		return
	}

	var response struct {
		Projects []struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
			Slug string      `json:"slug"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing response: %v\n", err)
		os.Exit(1)
	}

	if len(response.Projects) == 0 {
		fmt.Println("No projects found.")
		return
	}

	fmt.Printf("%-8s  %-14s  %s\n", "ID", "Slug", "Name")
	fmt.Printf("%-8s  %-14s  %s\n", "--------", "--------------", "----")
	for _, p := range response.Projects {
		fmt.Printf("%-8s  %-14s  %s\n", p.ID.String(), p.Slug, p.Name)
	}
}
