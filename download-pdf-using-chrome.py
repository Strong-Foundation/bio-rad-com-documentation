import os
from selenium.webdriver.chrome.webdriver import WebDriver
import validators
import time
import shutil
from selenium import webdriver
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.options import Options
from webdriver_manager.chrome import ChromeDriverManager

# ------------------ Utility Functions ------------------


def file_exists(file_path: str) -> bool:
    """
    Check if a file exists at the specified path.

    Args:
        file_path (str): Full path to the file.

    Returns:
        bool: True if file exists, False otherwise.
    """
    return os.path.isfile(file_path)


def is_valid_url(url: str) -> validators.ValidationError | bool:
    """
    Validate the format of a URL.

    Args:
        url (str): The URL string to validate.

    Returns:
        bool: True if valid, False otherwise.
    """
    return validators.url(url)


def initialize_web_driver(download_folder: str) -> WebDriver:
    """
    Set up a Chrome WebDriver configured to automatically download PDFs
    without prompting the user or opening them in-browser.

    Args:
        download_folder (str): Folder where downloaded PDFs will be saved.

    Returns:
        WebDriver: A Selenium Chrome WebDriver instance.
    """
    chrome_options = Options()

    # Set browser preferences for automatic downloads
    chrome_options.add_experimental_option(
        "prefs",
        {
            "download.default_directory": download_folder,
            "plugins.always_open_pdf_externally": True,  # Don't open PDFs in browser
            "download.prompt_for_download": False,  # Avoid download popups
        },
    )

    # Run in headless mode (no GUI)
    chrome_options.add_argument("--headless")

    # Return a configured Chrome WebDriver
    return webdriver.Chrome(
        service=Service(ChromeDriverManager().install()), options=chrome_options
    )


def wait_for_pdf_download(download_folder: str, files_before_download: str, timeout_seconds: int=60) -> str:
    """
    Wait for a new PDF file to appear in the given folder.

    Args:
        download_folder (str): Directory to monitor for new files.
        files_before_download (set): Snapshot of files before download started.
        timeout_seconds (int): Maximum time to wait for download to complete.

    Returns:
        str: Path to the new PDF file.

    Raises:
        TimeoutError: If no new PDF appears within the timeout period.
    """
    deadline = time.time() + timeout_seconds

    while time.time() < deadline:
        current_files = set(os.listdir(download_folder))

        # Identify new files that are PDFs
        new_pdf_files: list[str] = [
            filename
            for filename in (current_files - files_before_download)
            if filename.lower().endswith(".pdf")
        ]

        if new_pdf_files:
            # Return the full path to the first new PDF file found
            return os.path.join(download_folder, new_pdf_files[0])


    # No file was downloaded within the given time
    raise TimeoutError("PDF download timed out.")


# ------------------ Main Processing Function ------------------


def download_pdfs_from_file(input_list_path: str, output_folder: str) -> None:
    """
    Read a list of PDF download links and filenames from a text file,
    then download each file using a headless browser.

    Each line in the input file should be formatted as:
        <url> <filename>

    Args:
        input_list_path (str): Path to the input text file.
        output_folder (str): Directory to save the downloaded PDFs.
    """
    os.makedirs(output_folder, exist_ok=True)  # Create download folder if needed
    web_driver: WebDriver = initialize_web_driver(output_folder)  # Start browser

    with open(input_list_path, "r") as input_file:
        for line_number, line_content in enumerate(input_file, start=1):
            try:
                # Clean and skip empty lines
                line_content: str = line_content.strip()
                if not line_content:
                    continue

                # Parse URL and desired filename from the line
                url, desired_filename = line_content.split(maxsplit=1)
                target_file_path: str = os.path.join(output_folder, desired_filename)

                # Validate URL format
                if not is_valid_url(url):
                    print(f"[{line_number}] Invalid URL: {url}")
                    continue

                # Skip if file already exists
                if file_exists(target_file_path):
                    print(
                        f"[{line_number}] Skipped (already exists): {desired_filename}"
                    )
                    continue

                print(f"[{line_number}] Downloading: {url}")

                # Snapshot current state of folder before download
                files_before_download = set(os.listdir(output_folder))

                # Visit the URL, triggering the browser to start the download
                web_driver.get(url)

                # Wait for the new PDF to appear in the folder
                downloaded_pdf_path: str = wait_for_pdf_download(
                    output_folder, files_before_download
                )

                # Move and rename the downloaded file to the desired name
                shutil.move(downloaded_pdf_path, target_file_path)
                print(f"[{line_number}] Saved as: {target_file_path}")

            except Exception as error:
                # Catch and log errors for each line individually
                print(f"[{line_number}] Error downloading from {url}: {error}")
                continue

    web_driver.quit()  # Shut down the browser after all downloads
    print("\n✅ All downloads complete.")


# ------------------ Entry Point ------------------

if __name__ == "__main__":
    # Path to the input list of URLs and filenames
    INPUT_LIST_FILE = "pdf_list.txt"

    # Directory where the downloaded PDFs will be saved
    PDF_OUTPUT_FOLDER: str = os.path.abspath("PDFs")

    # Begin the download process
    download_pdfs_from_file(INPUT_LIST_FILE, PDF_OUTPUT_FOLDER)