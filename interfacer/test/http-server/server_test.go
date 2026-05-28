package test

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestHTTPServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Server tests")
}

var _ = Describe("HTTP Server", func() {
	It("should return plain text", func() {
		response := getPath("/smorgasbord", "plain")
		Expect(response).To(ContainSubstring("smörgåsbord"))
		Expect(response).ToNot(ContainSubstring("<a href"))
	})

	It("should return HTML text", func() {
		response := getPath("/smorgasbord", "html")
		Expect(response).To(ContainSubstring(
			"<a href=\"/http://localhost:4444/smorgasbord/another.html\">"))
	})

	It("should return the DOM", func() {
		response := getPath("/smorgasbord", "dom")
		Expect(response).To(ContainSubstring(
			"<div class=\"big_middle\">"))
	})

	It("should return a background image", func() {
		response := getPath("/smorgasbord", "html")
		Expect(response).To(ContainSubstring("background-image: url(data:image/jpeg"))
	})

	It("should block specified domains", func() {
		viper.Set(
			"http-server.blocked-domains",
			[]string{"[mail|accounts].google.com", "other"},
		)
		url := getBrowshServiceBase() + "/mail.google.com"
		client := &http.Client{}
		request, _ := http.NewRequest("GET", url, nil)
		response, _ := client.Do(request)
		contents, _ := ioutil.ReadAll(response.Body)
		Expect(string(contents)).To(ContainSubstring("Welcome to the Browsh HTML"))
	})

	It("should block specified user agents", func() {
		viper.Set(
			"http-server.blocked-user-agents",
			[]string{"MJ12bot", "other"},
		)
		url := getBrowshServiceBase() + "/example.com"
		client := &http.Client{}
		request, _ := http.NewRequest("GET", url, nil)
		request.Header.Add("User-Agent", "Blah blah MJ12bot etc")
		response, _ := client.Do(request)
		Expect(response.StatusCode).To(Equal(403))
	})

	It("should allow a blocked user agent to see the home page", func() {
		viper.Set(
			"http-server.blocked-user-agents",
			[]string{"MJ12bot", "other"},
		)
		url := getBrowshServiceBase()
		client := &http.Client{}
		request, _ := http.NewRequest("GET", url, nil)
		request.Header.Add("User-Agent", "Blah blah MJ12bot etc")
		response, _ := client.Do(request)
		Expect(response.StatusCode).To(Equal(200))
	})

	It("should handle MCP close requests safely without panic", func() {
		url := getBrowshServiceBase() + "/mcp"
		client := &http.Client{}
		
		// 1. list
		reqBody := `{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": {"name": "list_tabs", "arguments": {}}}`
		req, _ := http.NewRequest("POST", url, strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		Expect(err).To(BeNil())
		Expect(resp.StatusCode).To(Equal(200))
		respBody, _ := ioutil.ReadAll(resp.Body)
		_ = respBody

		// 2. open webpage
		reqBody2 := `{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": {"name": "open_webpage", "arguments": {"url": "http://localhost:4444/smorgasbord/index.html"}}}`
		req2, _ := http.NewRequest("POST", url, strings.NewReader(reqBody2))
		req2.Header.Set("Content-Type", "application/json")
		resp2, err := client.Do(req2)
		Expect(err).To(BeNil())
		Expect(resp2.StatusCode).To(Equal(200))

		// 3. close
		reqBody3 := `{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": {"name": "close", "arguments": {}}}`
		req3, _ := http.NewRequest("POST", url, strings.NewReader(reqBody3))
		req3.Header.Set("Content-Type", "application/json")
		resp3, err := client.Do(req3)
		Expect(err).To(BeNil())
		Expect(resp3.StatusCode).To(Equal(200))

		// 4. list again
		reqBody4 := `{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": {"name": "list_tabs", "arguments": {}}}`
		req4, _ := http.NewRequest("POST", url, strings.NewReader(reqBody4))
		req4.Header.Set("Content-Type", "application/json")
		resp4, err := client.Do(req4)
		Expect(err).To(BeNil())
		Expect(resp4.StatusCode).To(Equal(200))
		resp4Body, _ := ioutil.ReadAll(resp4.Body)
		// Should be valid JSON
		Expect(string(resp4Body)).To(ContainSubstring("jsonrpc"))
	})
})
