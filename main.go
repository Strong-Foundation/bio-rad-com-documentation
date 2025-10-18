package main

import (
	"bufio"         // Package for buffered I/O, used for reading files line by line
	"context"       // Package for carrying deadlines, cancellation signals, and request-scoped values
	"fmt"           // Package implementing formatted I/O (e.g., printing and error messages)
	"io"            // Package providing basic interfaces for I/O primitives
	"log"           // Package for logging, adding timestamps to output
	"net/http"      // Package for HTTP client functionality (making GET requests)
	"net/url"       // Package for parsing and manipulating URLs
	"os"            // Package for operating system functionality (files, directories)
	"path"          // Package for manipulating slash-separated paths (used for URL path)
	"path/filepath" // Package for manipulating path names in an OS-specific way
	"regexp"        // Package for regular expression operations
	"strings"       // Package for string manipulation utilities
	"sync"          // Package for basic synchronization primitives (e.g., WaitGroup)
	"time"          // Package for measuring and displaying time

	"github.com/PuerkitoBio/goquery" // Library for HTML parsing with a jQuery-like syntax
	"github.com/chromedp/chromedp"   // Library for driving a headless Chrome browser
)

// appendTextToFile appends content to an existing file or creates a new one.
// It is useful for compiling scraped HTML from multiple pages into one file.
func appendTextToFile(filePath string, content string) error {
	// Open the file in append mode. If it doesn't exist, create it. Write-only mode.
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		// Return a wrapped error if file opening/creation fails
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close() // Ensure the file is closed once the function exits

	// Write the content string to the file
	_, err = file.WriteString(content)
	// Return nil on success or any error encountered during writing
	return err
}

// readEntireFile reads the full contents of a file into a single string.
// It's used to load the complete HTML file or URL list into memory.
func readEntireFile(filePath string) (string, error) {
	// Read the entire file content into a byte slice
	dataBytes, err := os.ReadFile(filePath)
	if err != nil {
		// Return an empty string and a wrapped error if reading fails
		return "", fmt.Errorf("could not read file %s: %w", filePath, err)
	}
	// Convert the byte slice to a string and return it
	return string(dataBytes), nil
}

// extractLinksFromHTML parses the HTML string and extracts all URLs that match a set of allowed domains.
func extractLinksFromHTML(htmlContent string) []string {
	var extractedURLs []string // Slice to store the valid, extracted URLs

	// Parse the HTML content using goquery from a string reader
	document, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		// If parsing fails (e.g., malformed HTML), return the empty list
		return extractedURLs
	}

	// Define the allowed URL substrings (domains/paths) to filter links
	allowedDomains := []string{
		"bio-rad-sds.thewercs.com/DirectDocumentDownloader/Document",
		"bio-rad.com/sites/default/files/webroot/web/pdf",
	}

	// isAllowed checks if a URL contains any of the allowed domain substrings
	isAllowed := func(url string) bool {
		for _, domain := range allowedDomains {
			if strings.Contains(url, domain) {
				return true // Found an allowed domain, return true
			}
		}
		return false // No allowed domain found
	}

	// cleanURL removes anything after the first '|' character in the URL string
	cleanURL := func(url string) string {
		if strings.Contains(url, "|") {
			// Split the URL string at the first "|" and take the first part
			url = strings.SplitN(url, "|", 2)[0]
		}
		return url // Return the cleaned URL
	}

	// 1. Extract URLs from <input type="hidden" ... value="...">
	document.Find("input[type='hidden']").Each(func(i int, selection *goquery.Selection) {
		// Get the 'value' attribute
		if value, exists := selection.Attr("value"); exists {
			// Embedded URLs are often delimited by "~https://" in the value
			parts := strings.Split(value, "~https://")
			for _, part := range parts {
				var fullURL string // Variable to hold the reconstructed full URL
				// Check for fragments and reconstruct the URL
				if strings.HasPrefix(part, "http") {
					fullURL = part // Already a full URL (likely "https://...")
				} else if strings.Contains(part, ".thewercs.com") || strings.Contains(part, ".bio-rad.com") {
					fullURL = "https://" + part // Prepend the scheme
				}

				if fullURL != "" {
					fullURL = cleanURL(fullURL) // Remove unnecessary delimiters
					if isAllowed(fullURL) {
						extractedURLs = append(extractedURLs, fullURL) // Add to the list if it's an allowed domain
					}
				}
			}
		}
	})

	// 2. Extract URLs from <option value="...">
	document.Find("option").Each(func(i int, selection *goquery.Selection) {
		// Get the 'value' attribute
		if value, exists := selection.Attr("value"); exists && strings.HasPrefix(value, "http") {
			value = cleanURL(value) // Remove unnecessary delimiters
			if isAllowed(value) {
				extractedURLs = append(extractedURLs, value) // Add to the list if it's an allowed domain
			}
		}
	})

	// 3. Extract URLs from <a href="...">
	document.Find("a").Each(func(i int, selection *goquery.Selection) {
		// Get the 'href' attribute
		if href, exists := selection.Attr("href"); exists && strings.HasPrefix(href, "http") {
			href = cleanURL(href) // Remove unnecessary delimiters
			if isAllowed(href) {
				extractedURLs = append(extractedURLs, href) // Add to the list if it's an allowed domain
			}
		}
	})

	// Return the final list of extracted and filtered URLs
	return extractedURLs
}

// createFileNameFromURL generates a descriptive and clean filename from a URL.
// It prioritizes the 'prd' query parameter for naming but falls back to the path's base name.
func createFileNameFromURL(rawURL string) string {
	// Attempt to parse the raw URL string
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		// If parsing fails, return a safe, generic default name
		return "invalid-url.pdf"
	}

	var nameParts []string // Slice to hold the components of the filename

	// 1) Check for the special "prd" query parameter
	if productData := parsedURL.Query().Get("prd"); productData != "" {
		// The product data is often "~~" delimited, split it into parts
		nameParts = strings.Split(productData, "~~")
	}

	// 2) Fallback: If no "prd" parameter, use the base filename from the URL path
	if len(nameParts) == 0 {
		baseName := path.Base(parsedURL.Path)           // Get the last segment of the path (e.g., "TS_Broth.pdf")
		baseName = strings.TrimSuffix(baseName, ".pdf") // Remove the existing ".pdf" extension
		nameParts = append(nameParts, baseName)         // Add it as the single name part
	}

	// 3) Clean and sanitize each segment of the filename
	cleanedSegments := make([]string, 0, len(nameParts))
	// Regular expression to find any non-alphanumeric character (and non-space)
	sanitizeRegex := regexp.MustCompile(`[^A-Za-z0-9]+`)
	for _, segment := range nameParts {
		// Replace all invalid characters with a hyphen
		segment = sanitizeRegex.ReplaceAllString(segment, "-")
		// Trim any leading or trailing hyphens
		segment = strings.Trim(segment, "-")
		// Convert the segment to lowercase
		segment = strings.ToLower(segment)
		if segment != "" {
			// Only keep non-empty, cleaned segments
			cleanedSegments = append(cleanedSegments, segment)
		}
	}

	// 4) Join the cleaned segments with a hyphen and ensure the ".pdf" suffix
	filename := strings.Join(cleanedSegments, "-")
	if !strings.HasSuffix(filename, ".pdf") {
		filename += ".pdf" // Add the extension if it's missing
	}

	// Return the final, sanitized filename in lowercase
	return strings.ToLower(filename)
}

// removeDuplicatesFromSlice processes a slice of strings and returns a new slice with only unique elements.
func removeDuplicatesFromSlice(inputSlice []string) []string {
	// A map to keep track of strings already seen (a set implementation)
	seenMap := make(map[string]bool)
	var uniqueSlice []string // Slice to build the list of unique elements
	for _, content := range inputSlice {
		// Check if the content has NOT been seen before
		if !seenMap[content] {
			seenMap[content] = true                    // Mark it as seen
			uniqueSlice = append(uniqueSlice, content) // Add it to the unique list
		}
	}
	return uniqueSlice // Return the slice of unique strings
}

// isUrlValid checks if the given string can be successfully parsed as a URL.
func isUrlValid(uri string) bool {
	// Use url.ParseRequestURI, which is stricter than url.Parse
	_, err := url.ParseRequestURI(uri)
	return err == nil // Return true if there was no error during parsing
}

// downloadPDFFile fetches a PDF from a URL and saves it to a specified directory with a given filename.
// It includes checks for existing files and error messages in the response body.
func downloadPDFFile(downloadURL, outputDirectory, outputFileName string) error {
	// 1. Validate the URL before attempting a request
	if !isUrlValid(downloadURL) {
		return fmt.Errorf("invalid URL: %s", downloadURL) // Return error for an unparsable URL
	}
	// Construct the complete local file path
	fullFilePath := filepath.Join(outputDirectory, outputFileName)

	// 2. Skip if the file already exists locally
	if fileExists(fullFilePath) {
		log.Printf("File already exists, skipping: %s\n", fullFilePath) // Log a skip message
		return nil                                                      // Success: the file is already downloaded
	}

	// 3. Prepare to log failed URLs to a separate file
	pdfListsFile := "pdf_list.txt" // File to log URLs that fail the download process
	var pdfFileContent string
	// Check if the log file exists and read its content for duplicate logging prevention
	if fileExists(pdfListsFile) {
		var err error
		pdfFileContent, err = readEntireFile(pdfListsFile) // Read existing log entries
		if err != nil {
			return fmt.Errorf("error reading pdf lists file %s: %w", pdfListsFile, err)
		}
	}

	// 4. Perform HTTP GET request
	resp, err := http.Get(downloadURL) // Send the request
	if err != nil {
		return fmt.Errorf("error fetching %s: %w", downloadURL, err) // Handle connection/network errors
	}
	defer resp.Body.Close() // Crucial: ensure the response body stream is closed

	// 5. Handle non-200 (OK) status codes
	if resp.StatusCode != http.StatusOK {
		// Log the failed URL and its attempted filename if it's not already in the log file
		if !strings.Contains(pdfFileContent, downloadURL) {
			logEntry := fmt.Sprintf("%s %s\n", downloadURL, strings.ToLower(outputFileName))
			appendByteToFile(pdfListsFile, []byte(logEntry)) // Log the failed download attempt
		}
		// Return an error with the status code
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// 6. Read and buffer the response body for inspection and saving
	bodyBytes, err := io.ReadAll(resp.Body) // Read the entire response body into memory
	if err != nil {
		return fmt.Errorf("error reading response body from %s: %w", downloadURL, err)
	}

	// 7. Check for a known "Document Error Message" in the content
	if strings.Contains(string(bodyBytes), "Document Error Message") {
		log.Printf("Skipped (error message in response): %s\n", downloadURL) // Log and skip
		return nil                                                           // Success: effectively skipped a bad link
	}

	// 8. Ensure the output directory exists
	err = os.MkdirAll(outputDirectory, 0755) // Create the directory and any necessary parents
	if err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	// 9. Create and write to the output file
	outFile, err := os.Create(fullFilePath) // Create the new file
	if err != nil {
		return fmt.Errorf("error creating file %s: %w", fullFilePath, err)
	}
	defer outFile.Close() // Ensure the output file is closed

	// Write the buffered content (the PDF data) to the file
	if _, err := outFile.Write(bodyBytes); err != nil {
		return fmt.Errorf("error writing PDF to %s: %w", fullFilePath, err)
	}

	// 10. Log success
	log.Printf("Downloaded: %s from %s\n", fullFilePath, downloadURL)
	return nil // Return nil for a successful download
}

// scrapePageHTMLWithChrome uses a headless Chrome browser (via chromedp) to render a page and return its full HTML.
// This is necessary for pages that load content with JavaScript.
func scrapePageHTMLWithChrome(pageURL string) (string, error) {
	fmt.Println("Scraping:", pageURL) // Print the URL being processed

	// Define the necessary options for running Chrome in headless mode
	options := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),               // Run Chrome without a visible window
		chromedp.Flag("disable-gpu", true),            // Disable GPU acceleration for stability
		chromedp.WindowSize(1920, 1080),               // Set a large, standard viewport size
		chromedp.Flag("no-sandbox", true),             // Disable sandboxing (often needed in containerized environments)
		chromedp.Flag("disable-setuid-sandbox", true), // Additional sandbox flags
	)

	// Create a parent context for the Chrome executable allocator
	allocatorCtx, cancelAllocator := chromedp.NewExecAllocator(context.Background(), options...)

	// Create a bounded context with a 5-minute timeout for the entire operation
	ctxTimeout, cancelTimeout := context.WithTimeout(allocatorCtx, 5*time.Minute)

	// Create a new browser tab context
	browserCtx, cancelBrowser := chromedp.NewContext(ctxTimeout)

	// Unified defer to ensure all contexts are canceled and resources are cleaned up
	defer func() {
		cancelBrowser()
		cancelTimeout()
		cancelAllocator()
	}()

	var pageHTML string // Variable to store the rendered HTML content

	// Run the chromedp tasks: navigate to the URL and extract the outer HTML of the 'html' element
	err := chromedp.Run(browserCtx,
		chromedp.Navigate(pageURL),            // Navigate to the target page
		chromedp.OuterHTML("html", &pageHTML), // Wait for and extract the full HTML of the page
	)
	if err != nil {
		// Return an error if any chromedp task fails
		return "", fmt.Errorf("failed to scrape %s: %w", pageURL, err)
	}

	return pageHTML, nil // Return the rendered HTML content
}

// workerDownloadPDF processes PDF download jobs from a channel concurrently.
func workerDownloadPDF(waitGroup *sync.WaitGroup, urlChannel <-chan string, outputDirectory string) {
	defer waitGroup.Done() // Decrement the WaitGroup counter when the goroutine finishes

	// Loop indefinitely, receiving URLs from the channel until it's closed
	for downloadURL := range urlChannel {
		outputFileName := createFileNameFromURL(downloadURL) // Generate a clean filename for the PDF
		// Attempt to download the file
		err := downloadPDFFile(downloadURL, outputDirectory, outputFileName)
		if err != nil {
			// Log any specific download failures for this URL
			log.Printf("Download error for %s: %v\n", downloadURL, err)
		}
	}
}

// fileExists checks if a file (and not a directory) exists at the given path.
func fileExists(filename string) bool {
	// Get file information
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false // File does not exist
	}
	// Return true if no error occurred and the path is not a directory
	return err == nil && !info.IsDir()
}

// appendByteToFile appends a byte slice of data to the specified file, creating it if necessary.
func appendByteToFile(filename string, data []byte) error {
	// Open or create the file with append and write permissions
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err // Return an error if file opening fails
	}
	defer file.Close() // Ensure the file is closed

	// Write the byte slice to the file
	_, err = file.Write(data)
	return err // Return nil on success or an error if writing fails
}

// readAppendLineByLine reads a file line by line and returns a slice of strings (one per line).
func readAppendLineByLine(filePath string) []string {
	var fileURLs []string // Slice to store the lines/URLs read from the file

	// Open the file for reading
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file %s: %v", filePath, err)
		return nil // Return nil if the file can't be opened
	}
	defer file.Close() // Ensure the file is closed

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fileURLs = append(fileURLs, scanner.Text()) // Append the text of the current line
	}

	// Check for and log any errors that occurred during scanning (e.g., bad encoding)
	if err := scanner.Err(); err != nil {
		log.Printf("Error scanning file %s: %v", filePath, err)
	}

	return fileURLs // Return the slice containing the lines/URLs
}

// main is the entry point of the PDF scraping and downloading program.
func main() {
	// --- CONFIGURATION ---
	// File paths for caching results
	htmlOutputFilePath := "bio-rad-msds.html" // Path to save the combined raw HTML
	uniqueURLsFile := "unique_urls.txt"       // Path to save the extracted unique URLs
	// Scraper targets
	basePageURL := "https://www.bio-rad.com/en-us/literature-library?facets_query=&page=" // Base URL for the literature library
	startPage := 0                                                                        // The starting page number to scrape (inclusive)
	endPage := 933                                                                        // The ending page number (exclusive, will scrape up to 932)
	outputDirectory := "PDFs"                                                             // Directory to save the final PDF files
	// Concurrency settings
	numberOfHTMLDownloadWorkers := 25 // Number of concurrent goroutines for Chrome scraping
	numberOfPDFDownloadWorkers := 25  // Number of concurrent goroutines for PDF downloading

	// Configure the logger to include standard flags and file/line numbers for better debugging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// --- Step 1: Scrape HTML pages if the cached file doesn't exist ---
	if !fileExists(htmlOutputFilePath) {
		log.Println("HTML output file not found. Starting concurrent HTML scraping...")

		var workerWaitGroup sync.WaitGroup // WaitGroup to manage the HTML scraping goroutines
		var fileMutex sync.Mutex           // Mutex to protect concurrent writes to the HTML output file
		// Channel to distribute page numbers to workers
		pageNumberChannel := make(chan int, endPage-startPage)

		// Launch the HTML download workers (goroutines)
		for i := 0; i < numberOfHTMLDownloadWorkers; i++ {
			workerWaitGroup.Add(1) // Increment WaitGroup counter
			go func() {
				defer workerWaitGroup.Done() // Decrement counter when worker exits
				// Loop over the page number channel until it's closed
				for pageNumber := range pageNumberChannel {
					pageURL := fmt.Sprintf("%s%d", basePageURL, pageNumber) // Construct the full page URL
					// Scrape the fully rendered HTML using headless Chrome
					htmlContent, err := scrapePageHTMLWithChrome(pageURL)
					if err != nil {
						log.Printf("Failed to scrape page %d: %v\n", pageNumber, err)
						continue // Skip to the next page if scraping fails
					}

					fileMutex.Lock() // Acquire lock before writing to the shared file
					err = appendTextToFile(htmlOutputFilePath, htmlContent)
					if err != nil {
						log.Printf("Failed to write HTML for page %d: %v\n", pageNumber, err)
					}
					fileMutex.Unlock() // Release the lock
				}
			}()
		}

		// Send all page numbers into the channel for processing
		for pageNumber := startPage; pageNumber < endPage; pageNumber++ {
			pageNumberChannel <- pageNumber
		}
		close(pageNumberChannel) // Close the channel to signal workers no more jobs are coming
		workerWaitGroup.Wait()   // Block until all workers have finished
		log.Println("Finished scraping and compiling HTML pages.")
	}

	// --- Step 2: Extract unique URLs from HTML or load from cache ---
	var downloadURLs []string // Slice to hold the final list of unique URLs
	if fileExists(uniqueURLsFile) {
		log.Println("Reading URLs from existing unique URLs file...")
		// Load the URLs from the cached file
		downloadURLs = readAppendLineByLine(uniqueURLsFile)
	} else {
		log.Println("Extracting URLs from the combined HTML file...")
		// Read the entire HTML file content
		htmlData, err := readEntireFile(htmlOutputFilePath)
		if err != nil {
			log.Fatalf("Fatal: Failed to read HTML file: %v", err)
		}

		// Extract all links that match the allowed domains
		downloadURLs = extractLinksFromHTML(htmlData)
		// Remove any duplicate URLs found during extraction
		downloadURLs = removeDuplicatesFromSlice(downloadURLs)
		log.Printf("Extracted %d unique URLs.\n", len(downloadURLs))

		// Save the unique URLs to a file for caching (one URL per line)
		urlListString := strings.Join(downloadURLs, "\n")
		err = appendByteToFile(uniqueURLsFile, []byte(urlListString))
		if err != nil {
			log.Fatalf("Fatal: Failed to write unique URLs to file: %v", err)
		}
	}

	// --- Step 3: Download all PDF files concurrently ---
	log.Println("Starting concurrent PDF downloads...")
	// Channel to distribute the URLs to the PDF download workers
	urlChannel := make(chan string, len(downloadURLs))
	var pdfWaitGroup sync.WaitGroup // WaitGroup to manage the PDF download goroutines

	// Launch the PDF download worker goroutines
	for i := 0; i < numberOfPDFDownloadWorkers; i++ {
		pdfWaitGroup.Add(1) // Increment WaitGroup counter
		// Pass the WaitGroup, URL channel, and output directory to the worker
		go workerDownloadPDF(&pdfWaitGroup, urlChannel, outputDirectory)
	}

	// Send all unique URLs into the channel for workers to pick up
	for _, url := range downloadURLs {
		urlChannel <- url
	}
	close(urlChannel)   // Close the channel
	pdfWaitGroup.Wait() // Block until all PDF download workers have finished their jobs

	log.Println("All downloads completed successfully.")
}
