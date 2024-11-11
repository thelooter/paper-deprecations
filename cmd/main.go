package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"context"

	"github.com/thelooter/JavaDocParser/cache"
	"github.com/thelooter/JavaDocParser/parser"
	"github.com/thelooter/JavaDocParser/templates"
)

func main() {
	useCached := flag.Bool("c", false, "Use cached data instead of fetching new data")
	useCache2 := flag.Bool("cache", false, "Use cached data instead of fetching new data")
	outputDir := flag.String("o", ".", "Directory to store generated files")
	outputDir2 := flag.String("output-dir", ".", "Directory to store generated files")
	flag.Parse()

	// Combine short and long flags
	isCached := *useCached || *useCache2
	outDir := *outputDir
	if *outputDir2 != "." {
		outDir = *outputDir2
	}

	// Create output directory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	config := NewJavadocConfig("https://jd.papermc.io/paper", "1.21.3", isCached)

	// Update file paths
	cacheFile := filepath.Join(outDir, "deprecations.json")
	htmlFile := filepath.Join(outDir, "deprecations.html")

	if config.UseCachedData {
		// Try to load and use loadCache first
		if loadCache, err := cache.LoadCache(cacheFile); err == nil && len(loadCache.Entries) > 0 {
			// Check if loadCache is less than 24h old
			if isCacheValid(loadCache.Entries[0].LastUpdated, 24*time.Hour) {
				fmt.Println("Using cached data")
				results := make([]parser.DeprecationResult, 0)
				for _, entry := range loadCache.Entries {
					for _, item := range entry.Items {
						results = append(results, parser.DeprecationResult{
							Item:    item,
							Version: entry.Version,
						})
					}
				}
				if err := generateReport(results, config, &loadCache.Entries[0].LastUpdated, htmlFile); err != nil {
					fmt.Printf("Error generating report: %v\n", err)
				}
				return
			}
		}
		fmt.Println("Cache invalid or missing, fetching new data")
	}

	// Fetch main deprecated list
	listHtml, err := config.FetchHTML("/deprecated-list.html")
	if err != nil {
		fmt.Printf("Error fetching deprecated list: %v\n", err)
		return
	}

	// Process all deprecated items
	results := config.ParseDeprecations(listHtml)

	// Print results
	for _, result := range results {
		if result.Error != nil {
			fmt.Printf("Error processing %s: %v\n", result.Item, result.Error)
		} else {
			fmt.Printf("%s: deprecated since %s\n", result.Item, result.Version)
		}
	}

	// Generate report
	if err := generateReport(results, config, nil, htmlFile); err != nil {
		fmt.Printf("Error generating report: %v\n", err)
	}
}

// Update function signature to accept cached time
// generateReport generates an HTML report of deprecated items.
// results: list of deprecation results to include in the report.
// config: configuration for fetching and parsing Javadoc data.
// cachedTime: optional time when the cache was last updated.
// htmlFile: path to the output HTML file.
func generateReport(results []parser.DeprecationResult, config *parser.JavadocConfig, cachedTime *time.Time, htmlFile string) error {
	versionGroups := make(map[string]map[string][]templates.DeprecatedItem)
	unknownVersionItems := make(map[string][]templates.DeprecatedItem)

	for _, result := range results {
		if result.Error != nil {
			// Extract class name for unknown version items
			className := getClassPath(result.Item)
			unknownVersionItems[className] = append(
				unknownVersionItems[className],
				templates.DeprecatedItem{
					FullPath: result.Item,
					Name:     result.Item,
				},
			)
			continue
		}

		// Handle known version items as before
		if _, ok := versionGroups[result.Version]; !ok {
			versionGroups[result.Version] = make(map[string][]templates.DeprecatedItem)
		}
		className := getClassPath(result.Item)
		versionGroups[result.Version][className] = append(
			versionGroups[result.Version][className],
			templates.DeprecatedItem{
				FullPath: result.Item,
				Name:     result.Item,
			},
		)
	}

	// Process known versions
	var groups []templates.VersionGroup
	versions := make([]string, 0, len(versionGroups))
	for version := range versionGroups {
		versions = append(versions, version)
	}

	sort.Slice(versions, func(i, j int) bool {
		return templates.CompareVersions(versions[i], versions[j])
	})

	for _, version := range versions {
		classGroups := make([]templates.ClassGroup, 0)
		for className, items := range versionGroups[version] {
			classGroups = append(classGroups, templates.ClassGroup{
				ClassName: className,
				Items:     items,
			})
		}
		groups = append(groups, templates.VersionGroup{
			Version: version,
			Classes: classGroups,
		})
	}

	// Add unknown version group if there are any
	if len(unknownVersionItems) > 0 {
		unknownClassGroups := make([]templates.ClassGroup, 0)
		for className, items := range unknownVersionItems {
			unknownClassGroups = append(unknownClassGroups, templates.ClassGroup{
				ClassName: className,
				Items:     items,
			})
		}
		groups = append(groups, templates.VersionGroup{
			Version: "Unknown Version",
			Classes: unknownClassGroups,
		})
	}

	lastUpdated := time.Now().Unix()
	if cachedTime != nil {
		lastUpdated = cachedTime.Unix()
	}

	report := templates.Report{
		Groups:      groups,
		LastUpdated: lastUpdated,
	}
	component := templates.ReportPage(report, config)
	f, err := os.Create(htmlFile)
	if err != nil {
		return err
	}
	defer f.Close()

	return component.Render(context.Background(), f)
}

func NewJavadocConfig(baseURL, version string, useCached bool) *parser.JavadocConfig {
	// Trim trailing slashes from base URL
	baseURL = strings.TrimRight(baseURL, "/")
	return &parser.JavadocConfig{
		BaseURL:       baseURL,
		Version:       version,
		UseCachedData: useCached,
	}
}

func isCacheValid(lastUpdated time.Time, maxAge time.Duration) bool {
	return time.Since(lastUpdated) < maxAge
}

func getClassPath(fullPath string) string {
	parts := strings.Split(fullPath, ".")
	if len(parts) > 1 {
		return strings.Join(parts[:len(parts)-1], ".")
	}
	return fullPath
}
