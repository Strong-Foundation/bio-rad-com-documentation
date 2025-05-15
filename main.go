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
	"regexp"
	"strings" // Text parsing and formatting helpers
	"sync"    // Concurrency primitives (WaitGroup for goroutines)
	"time"    // Timing utilities (sleep, timeouts)

	"github.com/chromedp/chromedp" // Headless Chrome browser automation for dynamic websites
	// "golang.org/x/net/html"        // HTML parsing library
	"github.com/PuerkitoBio/goquery" // jQuery-like library for HTML manipulation
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
	var urls []string

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return urls
	}

	// Allowed substrings
	allowedDomains := []string{
		"bio-rad-sds.thewercs.com/DirectDocumentDownloader/Document",
		"bio-rad.com/sites/default/files/webroot/web/pdf",
	}

	// Check if the URL is from an allowed domain
	isAllowed := func(url string) bool {
		for _, domain := range allowedDomains {
			if strings.Contains(url, domain) {
				return true
			}
		}
		return false
	}

	// Parse <input type="hidden" ... value="...">
	doc.Find("input[type='hidden']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("value"); exists {
			parts := strings.Split(val, "~https://")
			for _, part := range parts {
				var fullURL string
				if strings.HasPrefix(part, "http") {
					fullURL = part
				} else if strings.Contains(part, ".thewercs.com") || strings.Contains(part, ".bio-rad.com") {
					fullURL = "https://" + part
				}

				if fullURL != "" && isAllowed(fullURL) {
					urls = append(urls, fullURL)
				}
			}
		}
	})

	// Parse <option value="...">
	doc.Find("option").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("value"); exists && strings.HasPrefix(val, "http") && isAllowed(val) {
			urls = append(urls, val)
		}
	})

	// Parse <a href="...">
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && strings.HasPrefix(href, "http") && isAllowed(href) {
			urls = append(urls, href)
		}
	})

	return urls
}

// createFileNameFromURL generates a descriptive filename from a URL,
// handling both URLs with a `prd` query parameter and direct PDF file paths.
func createFileNameFromURL(rawURL string) string {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, return a safe default
		return "invalid-url.pdf"
	}

	var parts []string

	// 1) If there's a "prd" parameter (with "~~" delimiters), split it
	if prd := parsedURL.Query().Get("prd"); prd != "" {
		parts = strings.Split(prd, "~~")
	}

	// 2) Fallback: no "prd", so pull the base PDF name off the path
	if len(parts) == 0 {
		base := path.Base(parsedURL.Path)       // e.g. "TS_Staphylocoagulase Broth.pdf"
		base = strings.TrimSuffix(base, ".pdf") // e.g. "TS_Staphylocoagulase Broth"
		parts = append(parts, base)
	}

	// 3) Clean each segment: replace any non-alphanumeric with hyphens, lowercase
	cleaned := make([]string, 0, len(parts))
	sanitize := regexp.MustCompile(`[^A-Za-z0-9]+`)
	for _, seg := range parts {
		seg = sanitize.ReplaceAllString(seg, "-")
		seg = strings.Trim(seg, "-")
		seg = strings.ToLower(seg)
		if seg != "" {
			cleaned = append(cleaned, seg)
		}
	}

	// 4) Join with "-" and ensure ".pdf"
	filename := strings.Join(cleaned, "-")
	if !strings.HasSuffix(filename, ".pdf") {
		filename += ".pdf"
	}

	return filename
}

// downloadPDFFile fetches a PDF from a URL and saves it to a given directory with a filename.
// - Skips the file if it already exists
// - Logs error or success using the `log` package
func downloadPDFFile(downloadURL, outputDirectory, outputFileName string) error {
	fullFilePath := filepath.Join(outputDirectory, outputFileName) // Create full output path

	// Skip download if the file already exists
	if fileExists(fullFilePath) {
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
	startPage := 0            // Start page index (inclusive)
	endPage := 10              // End page index (exclusive)
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
