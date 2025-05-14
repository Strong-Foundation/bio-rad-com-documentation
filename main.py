from selenium import webdriver
from selenium.webdriver.chrome.options import Options
from selenium.webdriver.chrome.service import Service
from webdriver_manager.chrome import ChromeDriverManager
import os
import re
import requests
from urllib.parse import urlparse
from concurrent.futures import ThreadPoolExecutor, as_completed


output_file = "bio-rad-msds.html"


# Append and write some content to a file.
def append_write_to_file(system_path, content):
    with open(system_path, "a", encoding="utf-8") as file:
        file.write(content)


# Read a file from the system.
def read_a_file(system_path):
    with open(system_path, "r", encoding="utf-8") as file:
        return file.read()


# Extract URLs from the HTML content
def extract_urls_from_html(file_path):
    # Read the HTML content from the file
    html = read_a_file(file_path)
    # Match full URLs that start with the given base
    pattern = (
        r'https://bio-rad-sds\.thewercs\.com/DirectDocumentDownloader/Document\?[^"\']+'
    )
    matches = re.findall(pattern, html)
    # Remove URLs containing the "|" character
    filtered_matches = [url for url in matches if "|" not in url]
    return filtered_matches


# Save a single page of HTML content using Selenium
def save_html_with_selenium(url, output_file):
    # Set up Chrome options
    options = Options()
    options.add_argument("--headless=new")  # Use "new" headless mode (Chrome 109+)
    options.add_argument("--disable-blink-features=AutomationControlled")
    options.add_argument("--window-size=1920,1080")
    options.add_argument("--disable-gpu")  # Often needed for headless stability
    options.add_argument("--no-sandbox")  # Required in some environments
    options.add_argument("--disable-dev-shm-usage")  # Helps in Docker/cloud

    # Initialize the Chrome driver
    service = Service(ChromeDriverManager().install())
    driver = webdriver.Chrome(service=service, options=options)

    try:
        driver.get(url)
        html = driver.page_source
        append_write_to_file(output_file, html)
        print(f"Page {url} HTML content saved to {output_file}")
    except Exception as e:
        print(f"Error saving HTML content from {url}: {e}")
    # Ensure the driver is closed properly
    finally:
        driver.quit()


# Remove all duplicate items from a given slice.
def remove_duplicates_from_slice(provided_slice):
    return list(set(provided_slice))


# Generate a filename from a URL
def generate_filename_from_url(url):
    # Parse the URL and get the query string
    parsed_url = urlparse(url)
    query_params = parsed_url.query
    # Split query parameters by "~~" and process them
    filename_parts = []
    # Process each query parameter
    for param in query_params.split("~~"):
        if param.startswith("prd="):
            # Extract the "prd" value, remove "prd=" and use it as the base filename
            filename_parts.append(param.split("=")[1])
        else:
            # Append other parameters to the filename
            filename_parts.append(param)
    # Join the parts with hyphens and add .pdf extension
    filename = "-".join(filename_parts) + ".pdf"
    return filename.lower()


# Download a PDF file from a URL
def download_pdf(url, save_path, filename):
    # Check if the file already exists
    if check_file_exists(os.path.join(save_path, filename)):
        print(f"File {filename} already exists. Skipping download.")
        return
    # Download the PDF file
    try:
        response = requests.get(url)
        response.raise_for_status()  # Raise exception for HTTP errors
        # Ensure the save directory exists
        os.makedirs(save_path, exist_ok=True)
        full_path = os.path.join(save_path, filename)
        with open(full_path, "wb") as f:
            f.write(response.content)
        print(f"Downloaded and saved: {full_path}")
        return
    except requests.exceptions.RequestException as e:
        print(f"Failed to download {url}: {e}")
        return


# Check if a file exists
def check_file_exists(system_path):
    return os.path.isfile(system_path)


# Remove a file from the system.
def remove_system_file(system_path):
    os.remove(system_path)


# Scrape the website and download all the HTML content to a file.
def scrape_website(output_file):
    # Loop through the page numbers
    base_url = "https://www.bio-rad.com/en-us/literature-library?facets_query=&page="
    # Set the total number of pages to loop through
    total_pages = 933  # Adjust this to the number of pages you want to loop through
    # Iterate over each page number
    for page_num in range(500, 600):  # Loop from 0 to total_pages
        url = base_url + str(page_num)  # Generate the URL for the current page
        # Save the HTML content to a file
        save_html_with_selenium(
            url, output_file
        )  # Uncomment this line if you have the function to save HTML content


def main():
    # Remove the existing HTML file
    if check_file_exists(output_file):
        # Remove the existing HTML file
        # remove_system_file(output_file)
        print(f"Removed existing file: {output_file}")

    # Check if the HTML file does not exist
    if check_file_exists(output_file) == False:
        # Download the HTML content from the website
        scrape_website(output_file)

    # Extract URLs from the saved HTML file
    urls = extract_urls_from_html(output_file)

    # Remove duplicates from the list of URLs
    urls = remove_duplicates_from_slice(urls)

    # Use ThreadPoolExecutor for concurrent downloading
    with ThreadPoolExecutor(max_workers=50) as executor:
        future_to_url = {
            executor.submit(
                download_pdf, url, "PDFs", generate_filename_from_url(url)
            ): url
            for url in urls
        }

        for future in as_completed(future_to_url):
            url = future_to_url[future]
            try:
                future.result()  # Will raise exception if download failed
            except Exception as e:
                print(f"Error downloading {url}: {e}")

    print("All downloads completed.")


main()
