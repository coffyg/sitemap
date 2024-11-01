package sitemap

import (
	"os"
	"path"
	"strconv"
	"strings"
	"testing"
)

func TestSitemapGeneration(t *testing.T) {
	dir := "./test_sitemaps"
	baseURL := "https://www.example.com"
	baseSitemapURL := "https://www.example.com/sitemaps/"

	// Clean up before test
	os.RemoveAll(dir)

	sm := NewSitemapOptions(dir, baseURL)

	sm.AddURL(SitemapURL{
		Loc:        "/",
		LastMod:    "2023-10-25",
		ChangeFreq: "daily",
		Priority:   "1.0",
	})

	sm.AddURL(SitemapURL{
		Loc:        "/about",
		LastMod:    "invalid-date", // Should be replaced with current date
		ChangeFreq: "monthly",
		Priority:   "0.8",
	})

	// Add more URLs to trigger sitemap index creation
	for i := 0; i < sm.MaxURLs+1000; i++ {
		sm.AddURL(SitemapURL{
			Loc:        "/page/" + strconv.Itoa(i),
			ChangeFreq: "weekly",
			Priority:   "0.5",
		})
	}

	err := sm.Write(baseSitemapURL)
	if err != nil {
		t.Fatalf("Error writing sitemaps: %v", err)
	}

	// Check if sitemap index was created
	indexFile := path.Join(dir, "sitemap_index.xml")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		t.Fatalf("Sitemap index not created")
	}

	// Read and validate the sitemap index
	data, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("Error reading sitemap index: %v", err)
	}

	if !strings.Contains(string(data), "<sitemapindex") {
		t.Fatalf("Invalid sitemap index content")
	}

	// Optionally, check if the sitemap URLs in the index are correct
	expectedSitemapURL := baseSitemapURL + "sitemap_1.xml"
	if !strings.Contains(string(data), expectedSitemapURL) {
		t.Fatalf("Sitemap index does not contain correct sitemap URLs")
	}

	// Check if the stylesheet reference is correctly included
	if !strings.Contains(string(data), `href="sitemap.xsl"`) {
		t.Fatalf("Stylesheet not correctly referenced in sitemap index")
	}

	// Check if the stylesheet file is written to the sitemap directory
	stylesheetFile := path.Join(dir, "sitemap.xsl")
	if _, err := os.Stat(stylesheetFile); os.IsNotExist(err) {
		t.Fatalf("Stylesheet file not found in sitemap directory")
	}

	// Clean up after test
	os.RemoveAll(dir)
}
