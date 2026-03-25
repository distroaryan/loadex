package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const asciiArt = `
 _                     _           
| |                   | |          
| |     ___   __ _  __| | _____  __
| |    / _ \ / _ \/ _` + "`" + ` |/ _ \ \/ /
| |___| (_) |  __/ (_| |  __/>  < 
\____/ \___/ \___|\__,_|\___/_/\_\

  Loadex - Go Load Balancer CLI 🚀`

var (
	adminURL  string
	targetURL string
	reqCount  int
)

var rootCmd = &cobra.Command{
	Use:   "loadex",
	Short: "CLI tool for managing loadex balancer",
	Long:  `CLI tool for querying health and managing backends in loadex daemon.`,
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Fetch live health map from loadex",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := http.Get(fmt.Sprintf("%s/api/health", adminURL))
		if err != nil {
			fmt.Println("Error connecting to loadex:", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		var healthMap map[string]bool
		if err := json.Unmarshal(body, &healthMap); err != nil {
			fmt.Println("Error parsing health map:", err)
			return
		}

		fmt.Println("\nLoadex Backend Health Status:")
		fmt.Println("--------------------------------------------------")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
		fmt.Fprintln(w, "SERVER\tSTATUS\t")
		fmt.Fprintln(w, "------\t------\t")
		for server, isHealthy := range healthMap {
			var status string
			if isHealthy {
				status = "\033[92m✅ Healthy\033[0m" // Bright green formatting
			} else {
				status = "\033[91m❌ Dead\033[0m"   // Bright red formatting
			}
			fmt.Fprintf(w, "%s\t%s\t\n", server, status)
		}
		w.Flush()
		fmt.Println("--------------------------------------------------")
	},
}

var killCmd = &cobra.Command{
	Use:   "kill-server",
	Short: "Kill a server via loadex daemon",
	Run: func(cmd *cobra.Command, args []string) {
		reqURL := fmt.Sprintf("%s/api/kill?url=%s", adminURL, url.QueryEscape(targetURL))
		req, _ := http.NewRequest("POST", reqURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("Error killing server:", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %s\nResponse: %s", resp.Status, string(body))
	},
}

var addCmd = &cobra.Command{
	Use:   "add-server",
	Short: "Add a new backend server to the pool",
	Run: func(cmd *cobra.Command, args []string) {
		reqURL := fmt.Sprintf("%s/api/add?url=%s", adminURL, url.QueryEscape(targetURL))
		req, _ := http.NewRequest("POST", reqURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("Error adding server:", err)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Status: %s\nResponse: %s", resp.Status, string(body))
	},
}

var makeReqCmd = &cobra.Command{
	Use:   "make-request",
	Short: "Make N requests to the load balancer",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Sending %d requests to %s...\n", reqCount, adminURL)
		success, fails := 0, 0
		for i := 0; i < reqCount; i++ {
			resp, err := http.Get(adminURL)
			if err != nil {
				fails++
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				success++
			} else {
				fails++
			}
		}
		fmt.Printf("Done! Success: %d, Fails: %d\n", success, fails)
	},
}

func init() {
	healthCmd.Flags().StringVarP(&adminURL, "url", "u", "http://localhost:8080", "URL of the loadex daemon")
	killCmd.Flags().StringVarP(&adminURL, "admin-url", "u", "http://localhost:8080", "URL of the loadex daemon")
	killCmd.Flags().StringVarP(&targetURL, "target", "t", "", "Target backend URL to kill")
	killCmd.MarkFlagRequired("target")

	addCmd.Flags().StringVarP(&adminURL, "admin-url", "u", "http://localhost:8080", "URL of the loadex daemon")
	addCmd.Flags().StringVarP(&targetURL, "target", "t", "", "Target backend URL to add")
	addCmd.MarkFlagRequired("target")

	makeReqCmd.Flags().StringVarP(&adminURL, "url", "u", "http://localhost:8080", "URL of the loadex proxy")
	makeReqCmd.Flags().IntVarP(&reqCount, "count", "c", 100, "Number of requests to make")

	rootCmd.AddCommand(healthCmd, killCmd, addCmd, makeReqCmd)
}

func main() {
	fmt.Println(asciiArt)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
