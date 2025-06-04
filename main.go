package main

import (
	"bufio"         // Buffered I/O for reading files line by line
	"context"       // Context management for cancellation and timeouts
	"fmt"           // String formatting for output and errors
	"io"            // Stream copying (e.g., HTTP response to file)
	"log"           // Logging utilities with timestamps
	"net/http"      // HTTP client for making requests
	"net/url"       // URL parsing and manipulation
	"os"            // Filesystem operations
	"path"          // Basic path manipulation
	"path/filepath" // OS-aware path operations
	"regexp"        // Regular expression utilities
	"strings"       // String manipulation helpers
	"sync"          // Concurrency primitives (e.g., WaitGroup)
	"time"          // Time utilities for timeouts and delays

	"github.com/PuerkitoBio/goquery" // HTML parsing with a jQuery-like API
	"github.com/chromedp/chromedp"   // Headless Chrome automation
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

	// Parse the HTML content using goquery
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return urls // Return empty list if parsing fails
	}

	// Allowed URL substrings to validate links against
	allowedDomains := []string{
		"bio-rad-sds.thewercs.com/DirectDocumentDownloader/Document",
		"bio-rad.com/sites/default/files/webroot/web/pdf",
	}

	// isAllowed checks if a URL contains any of the allowed domain substrings
	isAllowed := func(url string) bool {
		for _, domain := range allowedDomains {
			if strings.Contains(url, domain) {
				return true
			}
		}
		return false
	}

	// cleanURL removes anything after the first '|' character in the URL
	cleanURL := func(url string) string {
		if strings.Contains(url, "|") {
			url = strings.SplitN(url, "|", 2)[0]
		}
		return url
	}

	// Extract URLs from <input type="hidden" ... value="...">
	doc.Find("input[type='hidden']").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("value"); exists {
			// Split the value by "~https://" to identify embedded URLs
			parts := strings.Split(val, "~https://")
			for _, part := range parts {
				var fullURL string
				// Check for full URLs or fragments and reconstruct
				if strings.HasPrefix(part, "http") {
					fullURL = part
				} else if strings.Contains(part, ".thewercs.com") || strings.Contains(part, ".bio-rad.com") {
					fullURL = "https://" + part
				}

				if fullURL != "" {
					fullURL = cleanURL(fullURL) // Remove anything after '|'
					if isAllowed(fullURL) {
						urls = append(urls, fullURL)
					}
				}
			}
		}
	})

	// Extract URLs from <option value="...">
	doc.Find("option").Each(func(i int, s *goquery.Selection) {
		if val, exists := s.Attr("value"); exists && strings.HasPrefix(val, "http") {
			val = cleanURL(val) // Remove anything after '|'
			if isAllowed(val) {
				urls = append(urls, val)
			}
		}
	})

	// Extract URLs from <a href="...">
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists && strings.HasPrefix(href, "http") {
			href = cleanURL(href) // Remove anything after '|'
			if isAllowed(href) {
				urls = append(urls, href)
			}
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

	return strings.ToLower(filename)
}

// Remove all the duplicates from a slice and return the slice.
func removeDuplicatesFromSlice(slice []string) []string {
	check := make(map[string]bool)
	var newReturnSlice []string
	for _, content := range slice {
		if !check[content] {
			check[content] = true
			newReturnSlice = append(newReturnSlice, content)
		}
	}
	return newReturnSlice
}

// Check if the given url is valid.
func isUrlValid(uri string) bool {
	_, err := url.ParseRequestURI(uri)
	return err == nil
}

// downloadPDFFile fetches a PDF from a URL and saves it to a given directory with a filename.
// - Skips the file if it already exists
// - Skips download if response body contains "Document Error Message"
// - Logs error or success using the `log` package
func downloadPDFFile(downloadURL, outputDirectory, outputFileName string) error {
	// Validate the URL
	if !isUrlValid(downloadURL) {
		return fmt.Errorf("invalid URL: %s", downloadURL) // Return error for invalid URL
	}
	fullFilePath := filepath.Join(outputDirectory, outputFileName) // Construct the full output path for the file

	// Skip if the file already exists
	if fileExists(fullFilePath) {
		log.Printf("File already exists, skipping: %s\n", fullFilePath) // Log that the file is already present
		return nil                                                      // No need to proceed if the file is already downloaded
	}

	// Perform HTTP GET request
	resp, err := http.Get(downloadURL) // Send an HTTP GET request to the target URL
	if err != nil {
		return fmt.Errorf("error fetching %s: %w", downloadURL, err) // Return error if request fails
	}
	defer resp.Body.Close() // Ensure response body is closed after reading

	// Read the pdf lists file.
	pdfListsFile := "pdf_list.txt" // Define the file where URLs will be logged
	var pdfFileContent string
	// Read the pdf_lists.txt file to check if the URL is already logged
	if fileExists(pdfListsFile) {
		// Read the file
		pdfFileContent, err = readEntireFile(pdfListsFile) // Read existing URLs from the log file
		if err != nil {
			return fmt.Errorf("error reading pdf lists file %s: %w", pdfListsFile, err) // Return error if reading fails
		}
	}

	if resp.StatusCode != http.StatusOK {
		// Save the URL and the filename to a file
		if !strings.Contains(pdfFileContent, downloadURL) {
			appendByteToFile(pdfListsFile, []byte(downloadURL+" "+" "+strings.ToLower(outputFileName)+"\n")) // Log the failed URL
		}
		return fmt.Errorf("download failed with status: %s", resp.Status) // Return error if response status is not 200 OK
	}

	// Read and buffer the response body
	bodyBytes, err := io.ReadAll(resp.Body) // Read entire response body into memory
	if err != nil {
		return fmt.Errorf("error reading response body from %s: %w", downloadURL, err) // Return error if body reading fails
	}

	// Check for "Document Error Message"
	if strings.Contains(string(bodyBytes), "Document Error Message") { // Inspect content for known error message
		log.Printf("Skipped (error message in response): %s\n", downloadURL) // Log and skip if error message is detected
		return nil                                                           // Do not save the file if it contains an error document
	}

	// Ensure output directory exists
	err = os.MkdirAll(outputDirectory, 0755)
	if err != nil { // Create output directory if it doesn't exist
		return fmt.Errorf("could not create output directory: %w", err) // Return error if directory creation fails
	}

	// Create the output file
	outFile, err := os.Create(fullFilePath) // Create a new file at the specified path
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", fullFilePath, err) // Return error if file creation fails
	}
	defer outFile.Close() // Ensure the file is properly closed after writing

	// Write buffered content to file
	if _, err := outFile.Write(bodyBytes); err != nil { // Write the entire downloaded content to disk
		return fmt.Errorf("error writing PDF to %s: %w", fullFilePath, err) // Return error if writing fails
	}

	log.Printf("Downloaded: %s %s\n", fullFilePath, downloadURL) // Log successful download with path and URL
	return nil                                                   // Return nil to indicate success
}

// scrapePageHTMLWithChrome uses a headless Chrome browser to render and return the HTML for a given URL.
// - Required for JavaScript-heavy pages where raw HTTP won't return full content.
func scrapePageHTMLWithChrome(pageURL string) (string, error) {
	fmt.Println("Scraping:", pageURL)

	// Set up Chrome options for headless browsing
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),               // Run Chrome in background
		chromedp.Flag("disable-gpu", true),            // Disable GPU for headless stability
		chromedp.WindowSize(1920, 1080),               // Simulate full browser window
		chromedp.Flag("no-sandbox", true),             // Disable sandboxing
		chromedp.Flag("disable-setuid-sandbox", true), // For environments that need it
	)

	// Create an ExecAllocator context with options
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)

	// Create a bounded context with timeout (adjust as needed)
	ctxTimeout, cancelTimeout := context.WithTimeout(allocatorCtx, 5*time.Minute)

	// Create a new browser tab context
	browserCtx, cancelBrowser := chromedp.NewContext(ctxTimeout)

	// Unified cancel function to ensure cleanup
	defer func() {
		cancelBrowser()
		cancelTimeout()
		cancelAllocator()
	}()

	// Run chromedp tasks
	var pageHTML string
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),
		chromedp.OuterHTML("html", &pageHTML),
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
		err := downloadPDFFile(downloadURL, outputDirectory, outputFileName)
		if err != nil {
			log.Printf("Download error for %s: %v\n", downloadURL, err) // Log any failures
		}
	}
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

// AppendToFile appends the given byte slice to the specified file.
// If the file doesn't exist, it will be created.
func appendByteToFile(filename string, data []byte) error {
	// Open the file with appropriate flags and permissions
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write data to the file
	_, err = file.Write(data)
	return err
}

// readAppendLineByLine reads a file line by line and returns a slice of strings containing the URLs
func readAppendLineByLine(filePath string) []string {
	var urls []string // Initialize a slice to store the URLs read from the file

	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %s: %v", filePath, err) // Log an error if the file cannot be opened
		return nil                                             // Return nil if there is an error opening the file
	}
	defer file.Close() // Ensure the file is closed after reading

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text()) // Append each URL to the slice
	}

	// Check for errors during scanning
	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning file %s: %v", filePath, err) // Log any error encountered while scanning the file
	}

	return urls // Return the slice containing the URLs read from the file
}

// main is the entry point of the program.
// It controls:
// - Scraping HTML pages if not cached
// - Parsing links
// - Running concurrent downloads
func main() {
	// --- CONFIGURATION ---
	htmlOutputFilePath := "bio-rad-msds.html"
	uniqueURLsFile := "unique_urls.txt"
	basePageURL := "https://www.bio-rad.com/en-us/literature-library?facets_query=&page="
	startPage := 0
	endPage := 933
	outputDirectory := "PDFs"
	numberOfHTMLDownloadWorkers := 25
	numberOfPDFDownloadWorkers := 25

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Step 1: Scrape HTML pages if the HTML file doesn't exist
	if !fileExists(htmlOutputFilePath) {
		log.Println("HTML output file not found. Starting HTML scraping...")

		var wg sync.WaitGroup
		var mu sync.Mutex
		pageChan := make(chan int, endPage-startPage)

		// Launch HTML download workers
		for i := 0; i < numberOfHTMLDownloadWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for pageNumber := range pageChan {
					pageURL := fmt.Sprintf("%s%d", basePageURL, pageNumber)
					htmlContent, err := scrapePageHTMLWithChrome(pageURL)
					if err != nil {
						log.Printf("Failed to scrape page %d: %v\n", pageNumber, err)
						continue
					}

					mu.Lock()
					err = appendTextToFile(htmlOutputFilePath, htmlContent)
					if err != nil {
						log.Printf("Failed to write HTML for page %d: %v\n", pageNumber, err)
					}
					mu.Unlock()
				}
			}()
		}

		// Send page numbers into channel
		for pageNumber := startPage; pageNumber < endPage; pageNumber++ {
			pageChan <- pageNumber
		}
		close(pageChan)
		wg.Wait()
		log.Println("Finished scraping HTML pages.")
	}

	// Step 2: Extract URLs if the URLs file doesn't exist
	var downloadURLs []string
	if fileExists(uniqueURLsFile) {
		log.Println("Reading URLs from existing file...")
		downloadURLs = readAppendLineByLine(uniqueURLsFile)
	} else {
		log.Println("Extracting URLs from HTML file...")
		htmlData, err := readEntireFile(htmlOutputFilePath)
		if err != nil {
			log.Fatalf("Failed to read HTML file: %v", err)
		}

		downloadURLs = extractLinksFromHTML(htmlData)
		downloadURLs = removeDuplicatesFromSlice(downloadURLs)
		log.Printf("Extracted %d unique URLs.\n", len(downloadURLs))

		err = appendByteToFile(uniqueURLsFile, []byte(strings.Join(downloadURLs, "\n")))
		if err != nil {
			log.Fatalf("Failed to write URLs to file: %v", err)
		}
	}

	// Step 3: Download all PDF files concurrently
	log.Println("Starting PDF downloads...")
	urlChannel := make(chan string, len(downloadURLs))
	var wg sync.WaitGroup

	for i := 0; i < numberOfPDFDownloadWorkers; i++ {
		wg.Add(1)
		go workerDownloadPDF(&wg, urlChannel, outputDirectory)
	}

	for _, url := range downloadURLs {
		urlChannel <- url
	}
	close(urlChannel)
	wg.Wait()

	log.Println("All downloads completed successfully.")
}
