package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	logs "github.com/kpango/glg"
	"fmt"
)

var name string

const (
	// google_dns_url is the base URL for Google's DNS resolver.
	// The domain name is appended to this URL.
	google_dns_url = "https://dns.google.com/resolve?name="
)

// Response structure for Google DNS
type Response struct {
	Status   int  `json:"Status"`
	TC       bool `json:"TC"`
	RD       bool `json:"RD"`
	RA       bool `json:"RA"`
	AD       bool `json:"AD"`
	CD       bool `json:"CD"`
	Question []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
	} `json:"Question"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
}

func resolveDomain(domainName string, dnsServerURL string) (string, error) {
	url := dnsServerURL + domainName
	logs.Debugf("Querying DNS URL: %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to get response from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status code from %s: %d", url, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var dnsResponse Response
	err = json.Unmarshal(body, &dnsResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}

	if dnsResponse.Status != 0 { // 0 indicates success (NOERROR)
		return "", fmt.Errorf("DNS query failed with status: %d", dnsResponse.Status)
	}

	if len(dnsResponse.Answer) == 0 {
		return "", fmt.Errorf("no answer found for domain %s", domainName)
	}

	return dnsResponse.Answer[0].Data, nil
}

func init() {
	flag.StringVar(&name, "name", "ylck.me", "Domain name to resolve (e.g., -name example.com)")
	// flag.Parse() // Moved to main
}

func main() {
	flag.Parse() // Parse flags at the start of main
	t1 := time.Now()

	ip, err := resolveDomain(name, google_dns_url)
	if err != nil {
		logs.Errorf("Failed to resolve domain %s: %v", name, err)
		os.Exit(1) // Exit with error status
	}

	logs.Infof("%s : %s", name, ip)
	logs.Successf("Resolution completed in %s", time.Since(t1))
	// os.Exit(0) // Not strictly necessary, successful completion is exit code 0 by default
}
