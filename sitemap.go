package sitemap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

const (
	sitemapExt = ".xml"
	// Reduced max URLs by 1/3 for safety
	maxURLsPerSitemap = 33333
	// Stylesheet content
	sitemapXSL = `<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="2.0"
    xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
    xmlns:s="http://www.sitemaps.org/schemas/sitemap/0.9"
    xmlns:xhtml="http://www.w3.org/1999/xhtml">
    <xsl:output method="html" encoding="UTF-8" indent="yes"/>
    <xsl:template match="/">
        <html>
        <head>
            <title>Sitemap</title>
            <style type="text/css">
                body { font-family: Arial, sans-serif; }
                table { border-collapse: collapse; width: 100%; }
                th, td { text-align: left; padding: 8px; border-bottom: 1px solid #ddd; }
                tr:hover {background-color: #f5f5f5;}
                .alt-langs { font-size: 0.85em; color: #666; }
                .alt-langs a { margin-right: 6px; }
            </style>
        </head>
        <body>
            <h1>Sitemap</h1>
            <table>
                <tr>
                    <th>URL</th>
                    <th>Last Modified</th>
                    <th>Alternates</th>
                </tr>
                <xsl:for-each select="//s:url | //s:sitemap">
                    <tr>
                        <td><a href="{s:loc}"><xsl:value-of select="s:loc"/></a></td>
                        <td><xsl:value-of select="s:lastmod"/></td>
                        <td class="alt-langs">
                            <xsl:for-each select="xhtml:link[@rel='alternate']">
                                <a href="{@href}"><xsl:value-of select="@hreflang"/></a>
                            </xsl:for-each>
                        </td>
                    </tr>
                </xsl:for-each>
            </table>
        </body>
        </html>
    </xsl:template>
</xsl:stylesheet>
`
)

// AlternateLink represents an hreflang alternate link for internationalized sitemaps.
type AlternateLink struct {
	Hreflang string
	Href     string
}

// SitemapURL represents a single URL entry in the sitemap.
type SitemapURL struct {
	XMLName    xml.Name        `xml:"url"`
	Loc        string          `xml:"loc"`
	LastMod    string          `xml:"lastmod,omitempty"`
	ChangeFreq string          `xml:"changefreq,omitempty"`
	Priority   string          `xml:"priority,omitempty"`
	Alternates []AlternateLink `xml:"-"`
}

// URLSet represents a collection of SitemapURLs.
type URLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []SitemapURL `xml:"url"`
}

// Sitemap represents a sitemap file entry in the sitemap index.
type Sitemap struct {
	XMLName xml.Name `xml:"sitemap"`
	Loc     string   `xml:"loc"`
	LastMod string   `xml:"lastmod,omitempty"`
}

// SitemapIndex represents a collection of sitemaps.
type SitemapIndex struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	Xmlns    string    `xml:"xmlns,attr"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

// SitemapOptions holds configuration for generating sitemaps.
type SitemapOptions struct {
	MaxFileSize int
	MaxURLs     int
	Dir         string
	BaseURL     string
	URLs        []SitemapURL
	Stylesheet  string // Holds the stylesheet filename
}

// NewSitemapOptions initializes a new SitemapOptions instance.
func NewSitemapOptions(dir string, baseURL string) *SitemapOptions {
	return &SitemapOptions{
		MaxFileSize: 52428800, // 50MB
		MaxURLs:     maxURLsPerSitemap,
		Dir:         dir,
		BaseURL:     strings.TrimRight(baseURL, "/"),
		URLs:        []SitemapURL{},
		Stylesheet:  "sitemap.xsl", // Default stylesheet filename
	}
}

// AddURL adds a single SitemapURL to the sitemap, ensuring it's valid.
func (s *SitemapOptions) AddURL(url SitemapURL) {
	if url.LastMod == "" {
		url.LastMod = time.Now().UTC().Format("2006-01-02")
	} else {
		timeLastMod, err := time.Parse("2006-01-02", url.LastMod)
		if err != nil || timeLastMod.After(time.Now().UTC()) {
			url.LastMod = time.Now().UTC().Format("2006-01-02")
		}
	}
	if !strings.HasPrefix(url.Loc, "http://") && !strings.HasPrefix(url.Loc, "https://") {
		baseURL := s.BaseURL
		if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
			baseURL = "https://" + baseURL
		}
		baseURL = strings.TrimRight(baseURL, "/")
		if strings.HasPrefix(url.Loc, "/") {
			url.Loc = baseURL + url.Loc
		} else {
			url.Loc = baseURL + "/" + url.Loc
		}
	}
	s.URLs = append(s.URLs, url)
}

// AddURLs adds multiple SitemapURLs to the sitemap, ensuring they're valid.
func (s *SitemapOptions) AddURLs(urls []SitemapURL) {
	for _, url := range urls {
		s.AddURL(url)
	}
}

// Write generates the sitemap files based on the current URLs.
// baseSitemapURL is the base URL where the sitemap files will be accessible.
func (s *SitemapOptions) Write(baseSitemapURL string) error {
	// Ensure the directory exists
	if _, err := os.Stat(s.Dir); os.IsNotExist(err) {
		if err := os.MkdirAll(s.Dir, 0755); err != nil {
			return err
		}
	}

	// Write the stylesheet into the sitemap directory
	if err := s.writeStylesheet(); err != nil {
		return err
	}

	// Prepare URLs - resolve Loc and Alternate hrefs
	for i := range s.URLs {
		fullURL, err := s.resolveURL(s.URLs[i].Loc)
		if err != nil {
			return err
		}
		s.URLs[i].Loc = fullURL

		for j := range s.URLs[i].Alternates {
			href := s.URLs[i].Alternates[j].Href
			if href != "" && !strings.HasPrefix(href, "http://") && !strings.HasPrefix(href, "https://") {
				altURL, err := s.resolveURL(href)
				if err != nil {
					return err
				}
				s.URLs[i].Alternates[j].Href = altURL
			}
		}
	}

	// Decide whether to create a sitemap index or a single sitemap
	if len(s.URLs) <= s.MaxURLs {
		// Generate sitemap file
		err := s.writeSitemapFile("sitemap.xml", s.URLs)
		if err != nil {
			return err
		}
		// Validate the generated sitemap file
		return s.validateXMLFile(path.Join(s.Dir, "sitemap.xml"), false)
	} else {
		// Generate sitemap index
		err := s.writeSitemapIndex(baseSitemapURL)
		if err != nil {
			return err
		}
		// Validate the sitemap index and all sitemap files
		return s.validateSitemapIndexAndFiles()
	}
}

func (s *SitemapOptions) resolveURL(loc string) (string, error) {
	baseURL := s.BaseURL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(loc)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func (s *SitemapOptions) resolveSitemapURL(baseSitemapURL, sitemapName string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseSitemapURL, "/") + "/")
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(sitemapName)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func (s *SitemapOptions) writeStylesheet() error {
	filePath := path.Join(s.Dir, s.Stylesheet)
	return os.WriteFile(filePath, []byte(sitemapXSL), 0644)
}

// escapeXML escapes a string for safe use in XML content/attributes.
func escapeXML(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func (s *SitemapOptions) writeSitemapFile(filename string, urls []SitemapURL) error {
	hasAlternates := false
	for _, u := range urls {
		if len(u.Alternates) > 0 {
			hasAlternates = true
			break
		}
	}

	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	buffer.WriteString(fmt.Sprintf(`<?xml-stylesheet type="text/xsl" href="%s"?>`+"\n", s.Stylesheet))

	if hasAlternates {
		buffer.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\"\n")
		buffer.WriteString("        xmlns:xhtml=\"http://www.w3.org/1999/xhtml\">\n")
	} else {
		buffer.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	}

	for _, u := range urls {
		buffer.WriteString("  <url>\n")
		buffer.WriteString(fmt.Sprintf("    <loc>%s</loc>\n", escapeXML(u.Loc)))
		if u.LastMod != "" {
			buffer.WriteString(fmt.Sprintf("    <lastmod>%s</lastmod>\n", escapeXML(u.LastMod)))
		}
		if u.ChangeFreq != "" {
			buffer.WriteString(fmt.Sprintf("    <changefreq>%s</changefreq>\n", escapeXML(u.ChangeFreq)))
		}
		if u.Priority != "" {
			buffer.WriteString(fmt.Sprintf("    <priority>%s</priority>\n", escapeXML(u.Priority)))
		}
		for _, alt := range u.Alternates {
			buffer.WriteString(fmt.Sprintf("    <xhtml:link rel=\"alternate\" hreflang=\"%s\" href=\"%s\"/>\n",
				escapeXML(alt.Hreflang), escapeXML(alt.Href)))
		}
		buffer.WriteString("  </url>\n")
	}

	buffer.WriteString("</urlset>\n")

	filePath := path.Join(s.Dir, filename)
	return os.WriteFile(filePath, buffer.Bytes(), 0644)
}

func (s *SitemapOptions) writeSitemapIndex(baseSitemapURL string) error {
	index := SitemapIndex{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}

	fileCount := (len(s.URLs) + s.MaxURLs - 1) / s.MaxURLs
	for i := 0; i < fileCount; i++ {
		sitemapName := fmt.Sprintf("sitemap_%d.xml", i+1)
		start := i * s.MaxURLs
		end := start + s.MaxURLs
		if end > len(s.URLs) {
			end = len(s.URLs)
		}
		urlsSlice := s.URLs[start:end]
		err := s.writeSitemapFile(sitemapName, urlsSlice)
		if err != nil {
			return err
		}
		sitemapURL, err := s.resolveSitemapURL(baseSitemapURL, sitemapName)
		if err != nil {
			return err
		}
		index.Sitemaps = append(index.Sitemaps, Sitemap{
			Loc:     sitemapURL,
			LastMod: time.Now().UTC().Format("2006-01-02"),
		})
	}

	data, err := xml.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	// Add XML header and stylesheet with correct URL
	buffer := bytes.NewBufferString(xml.Header)
	buffer.WriteString(fmt.Sprintf(`<?xml-stylesheet type="text/xsl" href="%s"?>`+"\n", s.Stylesheet))
	buffer.Write(data)

	filePath := path.Join(s.Dir, "sitemap_index.xml")
	return os.WriteFile(filePath, buffer.Bytes(), 0644)
}

// validateXMLFile checks the given XML file we just wrote: it must be
// well-formed and unmarshal back into the exact struct it was generated
// from (URLSet or SitemapIndex), with the right root element and a non-empty
// <loc> on every entry. Pure encoding/xml — no libxml2/cgo (the lestrrat-go
// binding is abandoned and stopped compiling against libxml2 ≥ 2.14).
func (s *SitemapOptions) validateXMLFile(filePath string, isIndex bool) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read XML file for validation: %v", err)
	}

	if isIndex {
		var idx SitemapIndex
		if err := xml.Unmarshal(data, &idx); err != nil {
			return fmt.Errorf("failed to parse sitemap index XML: %v", err)
		}
		if idx.XMLName.Local != "sitemapindex" {
			return fmt.Errorf("XML validation failed: root element is <%s>, expected <sitemapindex>", idx.XMLName.Local)
		}
		if len(idx.Sitemaps) == 0 {
			return fmt.Errorf("XML validation failed: sitemap index has no <sitemap> entries")
		}
		for i, sm := range idx.Sitemaps {
			if strings.TrimSpace(sm.Loc) == "" {
				return fmt.Errorf("XML validation failed: sitemap index entry %d has an empty <loc>", i)
			}
		}
		return nil
	}

	var set URLSet
	if err := xml.Unmarshal(data, &set); err != nil {
		return fmt.Errorf("failed to parse sitemap XML: %v", err)
	}
	if set.XMLName.Local != "urlset" {
		return fmt.Errorf("XML validation failed: root element is <%s>, expected <urlset>", set.XMLName.Local)
	}
	for i, u := range set.URLs {
		if strings.TrimSpace(u.Loc) == "" {
			return fmt.Errorf("XML validation failed: url entry %d has an empty <loc>", i)
		}
	}
	return nil
}

func (s *SitemapOptions) validateSitemapIndexAndFiles() error {
	// Validate sitemap index
	indexFilePath := path.Join(s.Dir, "sitemap_index.xml")
	if err := s.validateXMLFile(indexFilePath, true); err != nil {
		return err
	}

	// Read the sitemap index to get the list of sitemaps
	indexData, err := os.ReadFile(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to read sitemap index for validation: %v", err)
	}

	var index SitemapIndex
	if err := xml.Unmarshal(indexData, &index); err != nil {
		return fmt.Errorf("XML unmarshalling failed for sitemap index: %v", err)
	}

	// Validate each sitemap file listed in the index
	for _, sitemap := range index.Sitemaps {
		// Extract the filename from the sitemap location
		sitemapURL, err := url.Parse(sitemap.Loc)
		if err != nil {
			return fmt.Errorf("invalid sitemap URL '%s': %v", sitemap.Loc, err)
		}
		sitemapFile := path.Base(sitemapURL.Path)
		sitemapFilePath := path.Join(s.Dir, sitemapFile)

		// Validate the sitemap file
		if err := s.validateXMLFile(sitemapFilePath, false); err != nil {
			return err
		}
	}
	return nil
}
