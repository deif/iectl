package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	version = "n/a"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version info on iectl",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		asJson, _ := cmd.Flags().GetBool("json")
		if asJson {
			return fmt.Errorf("version cant do --json")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		interactive, _ := cmd.Flags().GetBool("interactive")
		fmt.Printf("iectl %s, commit %s (%s)\n", version, commit, date)
		fmt.Printf("interactive terminal: %t\n", interactive)

		if runtime.GOOS == "windows" {
			doWindowsThings()
			return
		}

		checkGithubVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func checkGithubVersion() {
	fmt.Println()
	fmt.Println("Checking github for updates...")
	githubRelease := struct {
		// This url will be printed to the user
		// if a new version is found.
		URL string `json:"html_url"`
		// We will be checking against Tag.
		Tag string `json:"tag_name"`
		// We will be printing Name to the user.
		Name string `json:"name"`
	}{}

	resp, err := http.Get("https://api.github.com/repos/deif/iectl/releases/latest")
	if err != nil {
		fmt.Printf("unable to lookup github releases: %s", err)
		return
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&githubRelease)
	if err != nil {
		fmt.Printf("unable to parse response from github: %s", err)
		return
	}

	if githubRelease.Tag == "v"+version {
		fmt.Println("You are running the latest published version available.")
	} else {
		fmt.Printf("Version %s is available, fetch it from github:\n\n", githubRelease.Name)
		fmt.Println(githubRelease.URL)
	}

}

func doWindowsThings() {
	fmt.Println()
	fmt.Println("Checking winget for updates...")
	p, err := latestWingetVersion()
	if err != nil {
		fmt.Printf("unable to lookup latest iectl version: %s", err)
	}

	if p == version {
		fmt.Println("You are running the latest published version available.")
	} else {
		fmt.Printf("Version %s is available, update using:\n\n", p)
		fmt.Printf("winget upgrade --exact DEIF.iectl\n")
	}
}

func latestWingetVersion() (string, error) {
	c := exec.Command("winget", "search", "--exact", "DEIF.iectl")
	out, err := c.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("unable to pipe stdout: %w", err)
	}

	err = c.Start()
	if err != nil {
		return "", fmt.Errorf("unable to run winget: %w", err)
	}

	r, _ := regexp.Compile("Name *Id *Version *Source *$")

	scanner := bufio.NewScanner(out)
	for scanner.Scan() {
		if !r.MatchString(scanner.Text()) {
			continue
		}

		// We have arrived, eat a line
		scanner.Scan()
		scanner.Scan()

		// We should now be looking at
		// iectl    DEIF.iectl   0.0.2     winget
		fields := strings.Fields(scanner.Text())
		return fields[2], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading standard out: %w", err)
	}

	err = c.Wait()
	var targetError *exec.ExitError
	if errors.As(err, &targetError) {
		fmt.Println(targetError.Stderr)
		return "", fmt.Errorf("winget failed with non zero exitcode: %w", err)
	}
	if err != nil {
		return "", fmt.Errorf("winget failed: %w", err)
	}

	return "", fmt.Errorf("was unable to understand the output of winget")
}
