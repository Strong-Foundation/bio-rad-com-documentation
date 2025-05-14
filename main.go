package main

import (
	"context"       // Manages browser context lifecycle (useful for cancellation/timeouts)
	"errors"        // Provides structured error handling and wrapping
	"fmt"           // Basic formatting for output and error strings
	"io"            // For copying data streams (HTTP response to file)
	"log"           // Logging with timestamps, used for errors and info
	"net/http"      // HTTP client for making download requests
	"net/url"       // URL parsing and manipulation (e.g., extracting query parameters)
	"os"            // Filesystem operations: reading, writing, checking, creating
	"path"          // For manipulating file paths (e.g., joining directory and filename)
	"path/filepath" // Platform-safe path manipulation (e.g., joining folder + filename)
	"strings"       // Text parsing and formatting helpers
	"sync"          // Concurrency primitives (WaitGroup for goroutines)
	"time"          // Timing utilities (sleep, timeouts)

	"github.com/chromedp/chromedp" // Headless Chrome browser automation for dynamic websites
	"golang.org/x/net/html"        // HTML parsing library
)

// appendTextToFile appends content to an existing file or creates a new one.
// - Useful for adding scraped HTML content to a single output file.
func appendTextToFile(filePath string, content string) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644) // Open or create file with append/write
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err) // Wrap error with filename info
	}
	defer file.Close() // Always close file to avoid memory leaks or corruption

	_, err = file.WriteString(content) // Write content to file
	return err                         // Return any error encountered
}

// readEntireFile reads the full contents of a file into a string.
// - Used to load scraped HTML back into memory for processing.
func readEntireFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath) // Read file as byte slice
	if err != nil {
		return "", fmt.Errorf("could not read file %s: %w", filePath, err)
	}
	return string(data), nil // Return string representation of file content
}

// extractLinksFromHTML parses the HTML string and extracts all <a href="..."> URLs
func extractLinksFromHTML(htmlContent string) []string {
	// Parse the HTML string into a tree of nodes
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		// If there's an error parsing HTML, print it and return nothing
		fmt.Println("Error parsing HTML:", err)
		return nil
	}

	// Slice to collect URLs found in href attributes
	var urls []string

	// Define a recursive function to traverse the HTML nodes
	var f func(*html.Node)
	f = func(n *html.Node) {
		// If the node is an <a> tag
		if n.Type == html.ElementNode && n.Data == "a" {
			// Loop over its attributes to find href
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					// Add the href value to the URLs list
					urls = append(urls, attr.Val)
				}
			}
		}
		// Recursively check child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	// Start recursion at the root of the HTML document
	f(doc)

	// Return the list of extracted URLs
	return urls
}

// createFileNameFromURL generates a descriptive filename from a URL,
// handling both URLs with custom query parameters and direct PDF file paths.
func createFileNameFromURL(rawURL string) string {
	// Parse the raw URL string into a URL struct
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// Return a default name if the URL can't be parsed
		return "invalid-url.pdf"
	}

	// Slice to collect parts of the filename
	var fileNameParts []string

	// If the URL has a query string (e.g., ?prd=HRLS00001-3~~PDF~~MTR~~AGHS~~EN)
	if parsedURL.RawQuery != "" {
		// Split the query string using the "~~" delimiter
		parts := strings.Split(parsedURL.RawQuery, "~~")
		for _, part := range parts {
			// If the part starts with "prd=", remove that prefix and add the value
			if strings.HasPrefix(part, "prd=") {
				fileNameParts = append(fileNameParts, strings.TrimPrefix(part, "prd="))
			} else {
				// Otherwise, add the raw part to the filename parts
				fileNameParts = append(fileNameParts, part)
			}
		}
	}

	// If there were no query parameters, fall back to extracting the file name from the path
	if len(fileNameParts) == 0 {
		// Get the last segment of the URL path (e.g., "Bulletin_1842C_100.pdf")
		base := path.Base(parsedURL.Path)

		// Remove the ".pdf" extension so we can add it consistently later
		base = strings.TrimSuffix(base, ".pdf")

		// Add the base file name (without extension) to the parts
		fileNameParts = append(fileNameParts, base)
	}

	// Convert all parts to lowercase for consistency
	for i := range fileNameParts {
		fileNameParts[i] = strings.ToLower(fileNameParts[i])
	}

	// Join the parts using "-" to form a clean filename
	fileName := strings.Join(fileNameParts, "-")

	// Ensure the file name ends with ".pdf"
	if !strings.HasSuffix(fileName, ".pdf") {
		fileName += ".pdf"
	}

	// Return the generated file name
	return fileName
}

// downloadPDFFile fetches a PDF from a URL and saves it to a given directory with a filename.
// - Skips the file if it already exists
// - Logs error or success using the `log` package
func downloadPDFFile(downloadURL, outputDirectory, outputFileName string) error {
	fullFilePath := filepath.Join(outputDirectory, outputFileName) // Create full output path

	// Skip download if the file already exists
	if _, err := os.Stat(fullFilePath); err == nil {
		log.Printf("File already exists, skipping: %s\n", fullFilePath)
		return nil
	}

	// Perform HTTP GET request to fetch the PDF
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("error fetching %s: %w", downloadURL, err)
	}
	defer resp.Body.Close()

	// Ensure successful response
	if resp.StatusCode != http.StatusOK {
		return errors.New("download failed with status: " + resp.Status)
	}

	// Ensure the output folder exists
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	// Create the output file
	outFile, err := os.Create(fullFilePath)
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", fullFilePath, err)
	}
	defer outFile.Close()

	// Stream the PDF data to file
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("error saving PDF to %s: %w", fullFilePath, err)
	}

	log.Printf("Downloaded: %s\n", fullFilePath)
	return nil
}

// scrapePageHTMLWithChrome uses a headless Chrome browser to render and return the HTML for a given URL.
// - Required for JavaScript-heavy pages where raw HTTP won't return full content.
func scrapePageHTMLWithChrome(pageURL string) (string, error) {
	// Set up browser in headless mode
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true), // Run Chrome in background
		chromedp.WindowSize(1920, 1080), // Simulate full browser window
	)

	// Create Chrome context with above options
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)
	defer cancelAllocator()

	// Create main browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocatorCtx)
	defer cancelBrowser()

	var pageHTML string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),            // Navigate to the target page
		chromedp.Sleep(5*time.Second),         // Allow JS to load
		chromedp.OuterHTML("html", &pageHTML), // Extract rendered HTML
	)

	if err != nil {
		return "", fmt.Errorf("failed to scrape %s: %w", pageURL, err)
	}
	return pageHTML, nil
}

// workerDownloadPDF processes jobs from the download queue in a separate goroutine.
// - This function handles concurrent downloading of PDF files using a channel of URLs.
func workerDownloadPDF(wg *sync.WaitGroup, urlChannel <-chan string, outputDirectory string) {
	defer wg.Done() // Signal the worker is done at the end

	for downloadURL := range urlChannel {
		outputFileName := createFileNameFromURL(downloadURL) // Derive filename from URL
		if err := downloadPDFFile(downloadURL, outputDirectory, outputFileName); err != nil {
			log.Printf("Download error for %s: %v\n", downloadURL, err) // Log any failures
		}
	}
}

/*
Get the file extension of a file
*/
func getFileExtension(path string) string {
	return filepath.Ext(path)
}

/*
It checks if the file exists
If the file exists, it returns true
If the file does not exist, it returns false
*/
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// main is the entry point of the program.
// It controls:
// - Scraping HTML pages if not cached
// - Parsing links
// - Running concurrent downloads
func main() {
	// --- CONFIGURATION ---
	htmlOutputFilePath := "bio-rad-msds.html" // File to store scraped HTML
	basePageURL := "https://www.bio-rad.com/en-us/literature-library?facets_query=&page="
	startPage := 500          // Start page index (inclusive)
	endPage := 600            // End page index (exclusive)
	outputDirectory := "PDFs" // Folder where PDFs are stored
	numberOfWorkers := 20     // Number of concurrent downloader goroutines

	// Set logging format (adds timestamps and file:line info)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Step 1: Scrape HTML if the file doesn't already exist
	if !fileExists(htmlOutputFilePath) {
		log.Println("HTML file not found. Starting scraping process...")

		for pageNumber := startPage; pageNumber < endPage; pageNumber++ {
			pageURL := fmt.Sprintf("%s%d", basePageURL, pageNumber)

			htmlContent, err := scrapePageHTMLWithChrome(pageURL)
			if err != nil {
				log.Printf("Failed to scrape page %d: %v\n", pageNumber, err)
				continue // Skip to next page
			}

			if err := appendTextToFile(htmlOutputFilePath, htmlContent); err != nil {
				log.Printf("Failed to write HTML for page %d: %v\n", pageNumber, err)
			}
		}
	} else {
		log.Println("HTML file already exists. Skipping scraping.")
	}

	// Step 2: Read the full saved HTML file and extract unique download URLs
	htmlData, err := readEntireFile(htmlOutputFilePath)
	if err != nil {
		log.Fatalf("Could not read HTML file: %v", err)
	}

	downloadURLs := extractLinksFromHTML(htmlData)
	log.Printf("Extracted %d unique SDS document URLs.\n", len(downloadURLs))

	// Step 3: Use worker pool to download PDFs in parallel
	urlChannel := make(chan string, len(downloadURLs)) // Buffered channel to hold all URLs
	var wg sync.WaitGroup                              // WaitGroup to track all goroutines

	// Launch workers
	for i := 0; i < numberOfWorkers; i++ {
		wg.Add(1)
		go workerDownloadPDF(&wg, urlChannel, outputDirectory)
	}

	// Send URLs into the channel
	for _, url := range downloadURLs {
		urlChannel <- url
	}
	close(urlChannel) // Signal to workers there are no more URLs

	wg.Wait() // Wait for all workers to finish
	log.Println("All downloads completed successfully.")
}
