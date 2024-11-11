package parser

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// DeprecationResult Result struct to hold both item info and potential error
type DeprecationResult struct {
	Item    string
	Version string
	Error   error
}

type JavadocConfig struct {
	BaseURL       string // e.g. "https://jd.papermc.io/paper"
	Version       string // e.g. "1.21.3"
	UseCachedData bool
}

func (c *JavadocConfig) GetFullURL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := fmt.Sprintf("%s/%s%s", c.BaseURL, c.Version, path)
	fmt.Printf("Constructed URL: %s\n", url)
	return url
}

func (c *JavadocConfig) FetchHTML(path string) (string, error) {
	fullURL := c.GetFullURL(path)
	log.Printf("Fetching URL: %s\n", fullURL)

	resp, err := http.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("fetch failed for %s: %v", fullURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body failed for %s: %v", fullURL, err)
	}

	log.Printf("Fetched %d bytes from %s\n", len(body), fullURL)
	return string(body), nil
}

func (c *JavadocConfig) processDeprecatedItem(link, item string) DeprecationResult {
	log.Printf("Processing item: %s\n", item)
	result := DeprecationResult{Item: item}

	html, err := c.FetchHTML(link)
	if err != nil {
		result.Error = err
		return result
	}

	version := extractDeprecationVersion(html)
	if version == "" {
		// Instead of treating this as an error, assign "Unknown" as the version
		result.Version = "Unknown"
		return result
	}

	result.Version = version
	return result
}

func (c *JavadocConfig) ParseDeprecations(listHtml string) []DeprecationResult {
	log.Printf("Parsing deprecation list (%d bytes)", len(listHtml))

	itemRe := regexp.MustCompile(`<div class="col-summary-item-name[^"]*"><a href="([^"]+)">([^<]+)</a></div>`)
	matches := itemRe.FindAllStringSubmatch(listHtml, -1)
	log.Printf("Found %d deprecated items\n", len(matches))

	// Create channels and WaitGroup
	resultsChan := make(chan DeprecationResult, len(matches))
	var wg sync.WaitGroup

	// Number of worker goroutines
	numWorkers := 8
	itemsPerWorker := (len(matches) + numWorkers - 1) / numWorkers

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Calculate start and end indices for this worker
			start := workerID * itemsPerWorker
			end := start + itemsPerWorker
			if end > len(matches) {
				end = len(matches)
			}

			// Process only this worker's portion of matches
			for j := start; j < end; j++ {
				match := matches[j]
				log.Printf("Worker %d processing item %d/%d: %s\n", workerID, j+1, len(matches), match[2])
				result := c.processDeprecatedItem(match[1], match[2])
				resultsChan <- result
			}
		}(i)
	}

	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []DeprecationResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results
}

// Extract version from @Deprecated annotation block
func extractDeprecationVersion(html string) string {
	// Look for deprecation annotation block with since value
	sinceRe := regexp.MustCompile(`@Deprecated\([^)]*since="([^"]+)"`)
	match := sinceRe.FindStringSubmatch(html)

	if len(match) > 1 {
		log.Printf("Found deprecation version: %s\n", match[1])
		return match[1]
	}

	// Fallback: try to find any since="" value after a Deprecated href
	fallbackRe := regexp.MustCompile(`href="[^"]*Deprecated.html#since[^"]*"[^>]*>[^<]*</a>="([^"]+)"`)
	match = fallbackRe.FindStringSubmatch(html)

	if len(match) > 1 {
		log.Printf("Found deprecation version (fallback): %s\n", match[1])
		return match[1]
	}

	log.Println("No deprecation version found in HTML")
	return ""
}
