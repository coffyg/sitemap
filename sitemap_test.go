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
		t.Fatalf("Error writing sitemaps: %+v", err)
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

func TestSitemapWithHreflang(t *testing.T) {
	dir := "./test_sitemaps_hreflang"
	baseURL := "soulkyn.com"
	baseSitemapURL := "https://soulkyn.com/sitemaps/"

	// Clean up before test
	os.RemoveAll(dir)

	sm := NewSitemapOptions(dir, baseURL)

	langs := []string{"en-us", "fr-fr", "de-de", "es-es", "ja-jp"}
	basePath := "/"

	// Build alternates for all languages
	alternates := make([]AlternateLink, 0, len(langs))
	for _, lang := range langs {
		alternates = append(alternates, AlternateLink{
			Hreflang: lang,
			Href:     "https://soulkyn.com/l/" + lang,
		})
	}

	// Add a URL for each language with all alternates
	for _, lang := range langs {
		sm.AddURL(SitemapURL{
			Loc:        "/l/" + lang + basePath,
			LastMod:    "2026-02-19",
			ChangeFreq: "daily",
			Priority:   "1.0",
			Alternates: alternates,
		})
	}

	// Add a URL without alternates to test mixed mode
	sm.AddURL(SitemapURL{
		Loc:        "/about",
		LastMod:    "2026-01-01",
		ChangeFreq: "monthly",
		Priority:   "0.5",
	})

	err := sm.Write(baseSitemapURL)
	if err != nil {
		t.Fatalf("Error writing sitemaps: %+v", err)
	}

	// Read the generated sitemap
	sitemapFile := path.Join(dir, "sitemap.xml")
	data, err := os.ReadFile(sitemapFile)
	if err != nil {
		t.Fatalf("Error reading sitemap: %v", err)
	}
	content := string(data)

	// Check xmlns:xhtml declaration
	if !strings.Contains(content, `xmlns:xhtml="http://www.w3.org/1999/xhtml"`) {
		t.Fatalf("Missing xhtml namespace declaration")
	}

	// Check xhtml:link elements
	if !strings.Contains(content, `<xhtml:link rel="alternate" hreflang="en-us" href="https://soulkyn.com/l/en-us"/>`) {
		t.Fatalf("Missing xhtml:link for en-us")
	}
	if !strings.Contains(content, `<xhtml:link rel="alternate" hreflang="ja-jp" href="https://soulkyn.com/l/ja-jp"/>`) {
		t.Fatalf("Missing xhtml:link for ja-jp")
	}

	// Check that each localized URL has ALL alternates
	// Count how many times the en-us alternate appears (should be once per lang URL = 5 times)
	enUsCount := strings.Count(content, `hreflang="en-us"`)
	if enUsCount != 5 {
		t.Fatalf("Expected 5 en-us alternates (one per lang URL), got %d", enUsCount)
	}

	// Verify the about page has no alternates
	aboutIdx := strings.Index(content, "soulkyn.com/about")
	if aboutIdx == -1 {
		t.Fatalf("About page not found in sitemap")
	}

	t.Logf("Sitemap with hreflang generated and validated successfully")

	// Clean up after test
	os.RemoveAll(dir)
}
