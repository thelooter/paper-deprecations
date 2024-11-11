package parser

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/html"
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
	fmt.Printf("Fetching URL: %s\n", fullURL)

	resp, err := http.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("fetch failed for %s: %v", fullURL, err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("Error closing response body: %v\n", err)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body failed for %s: %v", fullURL, err)
	}

	fmt.Printf("Fetched %d bytes from %s\n", len(body), fullURL)
	return string(body), nil
}

func (c *JavadocConfig) processDeprecatedItem(link, item string) DeprecationResult {
	fmt.Printf("Processing item: %s\n", item)
	result := DeprecationResult{Item: item}

	fetchedHTML, err := c.FetchHTML(link)
	if err != nil {
		result.Error = err
		return result
	}

	version := extractDeprecatedSince(fetchedHTML, item)
	if version == "" {
		// Instead of treating this as an error, assign "Unknown" as the version
		result.Version = "Unknown"
		return result
	}

	result.Version = version
	return result
}

func (c *JavadocConfig) ParseDeprecations(listHtml string) []DeprecationResult {
	fmt.Printf("Parsing deprecation list (%d bytes)", len(listHtml))

	itemRe := regexp.MustCompile(`<div class="col-summary-item-name[^"]*"><a href="([^"]+)">([^<]*)(?:<wbr>)?([^<]+)</a></div>`)
	matches := itemRe.FindAllStringSubmatch(listHtml, -1)
	fmt.Printf("Found %d deprecated items\n", len(matches))

	//Iterate over matches and print out the item name and link
	for _, match := range matches {
		fullText := match[2] + match[3]
		fmt.Printf("Item: %s, Link: %s\n", fullText, match[1])
	}

	// Create channels and WaitGroup
	resultsChan := make(chan DeprecationResult, len(matches))
	var wg sync.WaitGroup

	// Number of worker goroutines
	numWorkers := 10
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
				fullText := match[2] + match[3]
				workerItemCount := end - start
				fmt.Printf("Worker %d processing item %d/%d: %s\n", workerID, j-start+1, workerItemCount, fullText)
				result := c.processDeprecatedItem(match[1], fullText)
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

func extractDeprecatedSince(fullHTML, elementID string) string {
	fmt.Printf("Extracting deprecated since value for element with ID: %q\n", elementID)

	// Only use the last part of the elementID
	itemName := elementID[strings.LastIndex(elementID, ".")+1:]

	doc, err := html.Parse(strings.NewReader(fullHTML))
	if err != nil {
		fmt.Printf("Error parsing HTML: %v\n", err)
		return ""
	}
	fmt.Println("Successfully parsed HTML document")

	elementNode := findElementNodeByID(doc, itemName)
	if elementNode == nil {
		fmt.Printf("No element found with ID: %q\n", itemName)
		return ""
	}
	fmt.Printf("Found element with ID: %q\n", itemName)

	return extractDeprecatedSinceFromNode(elementNode)
}

func findElementNodeByID(n *html.Node, id string) *html.Node {
	fmt.Printf("Searching for element with ID: %q\n", id)
	var found *html.Node
	var traverse func(n *html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "section" {
				for _, attr := range n.Attr {
					if attr.Key == "id" {
						if attr.Val == id {
							fmt.Printf("Found matching section node\n")
							found = n
							return
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return found
}
func extractDeprecatedSinceFromNode(n *html.Node) string {
	fmt.Println("Extracting deprecated since value from node")

	// Find the annotations span first
	var findAnnotationsSpan func(*html.Node) *html.Node
	findAnnotationsSpan = func(n *html.Node) *html.Node {
		if n.Type == html.ElementNode && n.Data == "span" {
			for _, attr := range n.Attr {
				if attr.Key == "class" && attr.Val == "annotations" {
					fmt.Println("Found annotations span")
					return n
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if result := findAnnotationsSpan(c); result != nil {
				return result
			}
		}
		return nil
	}

	annotationsSpan := findAnnotationsSpan(n)
	if annotationsSpan == nil {
		fmt.Println("No annotations span found")
		return ""
	}

	// Now find the since link and extract the version that follows it
	var version string
	var foundSince bool
	for c := annotationsSpan.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "a" {
			for _, attr := range c.Attr {
				if attr.Key == "href" && strings.Contains(attr.Val, "#since()") {
					fmt.Println("Found since link")
					foundSince = true
					continue
				}
			}
		}
		// After finding the since link, look for the version in text nodes
		if foundSince && c.Type == html.TextNode {
			text := strings.TrimSpace(c.Data)
			if strings.Contains(text, "=") {
				version = strings.Trim(strings.Split(text, "=")[1], "\"") // Remove quotes
				version = strings.TrimSuffix(version, "\")")              // Remove trailing parenthesis
				version = strings.TrimSuffix(version, "\",")              // Remove trailing comma
				fmt.Printf("Found version: %s\n", version)
				return version
			}
		}
	}

	return ""
}
