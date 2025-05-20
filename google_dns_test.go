package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveDomainSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query parameter
		queryName := r.URL.Query().Get("name")
		if queryName != "example.com" {
			http.Error(w, fmt.Sprintf("unexpected domain in query: got %s, want example.com", queryName), http.StatusBadRequest)
			return
		}

		// Check that the path is /resolve (if dnsServerURL is "http://<server_ip_or_host>/resolve?")
		// For this test, we assume the mock server URL itself is the full base, and "name" is appended.
		// If the dnsServerURL for resolveDomain includes a path, this check needs adjustment.
		// Our current resolveDomain function takes dnsServerURL and appends domainName,
		// so for httptest.Server, server.URL will be like "http://127.0.0.1:XXXX"
		// and resolveDomain will call "http://127.0.0.1:XXXXexample.com".
		// This is not ideal. The google_dns_url constant is "https://dns.google.com/resolve?name=".
		// So, resolveDomain gets dnsServerURL = "http://mockserver/prefix?" and appends "name=example.com" effectively.
		// Let's adjust the test to mimic this behavior. The mock server URL will be server.URL + "/resolve?name="

		// Correcting the expectation for the mock server:
		// The resolveDomain function receives the base part of the URL (up to and including "?name=")
		// and then appends the domain name.
		// So, if server.URL is "http://127.0.0.1:12345", we should pass server.URL + "?name=" to resolveDomain.
		// The request r.URL.Path will be "/" and r.URL.RawQuery will be "name=example.com".

		if r.URL.Path != "/" && !strings.HasSuffix(r.URL.Path, "/resolve") { // Allow flexibility if server URL has a path
			// If the server URL from httptest is "http://ip:port", path is "/".
			// If it's "http://ip:port/resolve", path is "/resolve".
			// The original google_dns_url has "/resolve?name=".
			// Our refactored resolveDomain takes the full base URL string.
			// So, the mock server handler should expect the request at its root,
			// and the query param "name" is what matters.
		}
		
		// The important part is that `r.URL.Query().Get("name")` is correct.

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"Status":0,"TC":false,"RD":true,"RA":true,"AD":false,"CD":false,"Question":[{"name":"example.com","type":1}],"Answer":[{"name":"example.com","type":1,"TTL":3600,"data":"1.2.3.4"}]}`)
	}))
	defer server.Close()

	// Construct the mock server URL to match how google_dns_url is defined (ends with "?name=")
	// The resolveDomain function will append the actual domain to this.
	// The original google_dns_url is "https://dns.google.com/resolve?name=".
	// The mock server handler is listening at server.URL (e.g. http://127.0.0.1:XYZ).
	// We want resolveDomain to hit http://127.0.0.1:XYZ/resolve?name=example.com.
	// So, the dnsServerURL argument to resolveDomain should be "http://127.0.0.1:XYZ/resolve?name=".
	mockDNSURL := server.URL + "/resolve?name="

	ip, err := resolveDomain("example.com", mockDNSURL)

	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	expectedIP := "1.2.3.4"
	if ip != expectedIP {
		t.Errorf("Expected IP %s, but got %s", expectedIP, ip)
	}
}

// TODO: Add TestResolveDomainErrorCases to cover:
// - Server error (500)
// - Malformed JSON response
// - JSON response with Status != 0
// - JSON response with no Answer
// - Network error (server not reachable - harder to mock reliably without more complex setup)

func TestResolveDomainQueryFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryName := r.URL.Query().Get("name")
		if queryName != "baddomain.com" {
			http.Error(w, "Unexpected domain", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Simulate a SERVFAIL or NXDOMAIN type of error from Google DNS
		fmt.Fprintln(w, `{"Status": 2, "TC":false,"RD":true,"RA":true,"AD":false,"CD":false,"Question":[{"name":"baddomain.com","type":1}],"Comment":"DNS query status: 2 (SERVFAIL)."}`)
	}))
	defer server.Close()

	mockDNSURL := server.URL + "/resolve?name="
	ip, err := resolveDomain("baddomain.com", mockDNSURL)

	if err == nil {
		t.Errorf("Expected an error due to DNS query failure, but got nil. IP: %s", ip)
	}

	if ip != "" {
		t.Errorf("Expected empty IP on DNS query failure, got %s", ip)
	}
    // Check for specific error message if desired
    expectedErrorMsg := "DNS query failed with status: 2"
    if err != nil && !strings.Contains(err.Error(), expectedErrorMsg) {
        t.Errorf("Expected error message to contain '%s', got '%s'", expectedErrorMsg, err.Error())
    }
}

func TestInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryName := r.URL.Query().Get("name")
		if queryName != "invalidjson.com" {
			http.Error(w, "Unexpected domain for invalid JSON test", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `this is not valid json`) // Malformed JSON
	}))
	defer server.Close()

	mockDNSURL := server.URL + "/resolve?name="
	ip, err := resolveDomain("invalidjson.com", mockDNSURL)

	if err == nil {
		t.Errorf("Expected an error due to invalid JSON response, but got nil. IP: %s", ip)
	}
	if ip != "" {
		t.Errorf("Expected empty IP when JSON response is invalid, got %s", ip)
	}
	// Check for a JSON parsing error message
	// The exact message can vary based on Go's JSON parser version, so a partial match is good.
	// Example: "failed to unmarshal JSON response: invalid character 'h' in literal true (expecting 'r')"
	// Or "failed to unmarshal JSON response: invalid character 't' looking for beginning of value"
	if err != nil && !strings.Contains(err.Error(), "failed to unmarshal JSON response") {
		t.Errorf("Expected error message to indicate JSON unmarshal failure, got '%s'", err.Error())
	}
}

func TestEmptyDomainName(t *testing.T) {
	// For an empty domain name, Google DNS returns Status:0 but an empty Answer array.
	// Our resolveDomain function should then return a "no answer found" error.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryName := r.URL.Query().Get("name")
		if queryName != "" { // Expecting an empty domain name
			http.Error(w, fmt.Sprintf("Expected empty domain name, got '%s'", queryName), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Simulate Google's response for an empty 'name' query
		fmt.Fprintln(w, `{"Status":0,"TC":false,"RD":true,"RA":true,"AD":false,"CD":false,"Question":[{"name":"","type":1}],"Answer":[],"Comment":"No A records found for ''"}`)
	}))
	defer server.Close()

	mockDNSURL := server.URL + "/resolve?name="
	ip, err := resolveDomain("", mockDNSURL) // Call with empty domain

	if err == nil {
		t.Errorf("Expected an error for empty domain name, but got nil. IP: %s", ip)
	}
	if ip != "" {
		t.Errorf("Expected empty IP for empty domain name, got %s", ip)
	}
	// Check for the specific "no answer found" error because Google DNS actually "succeeds" but returns no records.
	expectedErrorMsg := "no answer found for domain " // The domain name is empty
	if err != nil && !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected error message '%s', got '%s'", expectedErrorMsg, err.Error())
	}
}
