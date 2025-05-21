import os
import validators
import time
import shutil
from selenium import webdriver
from selenium.webdriver.chrome.service import Service
from selenium.webdriver.chrome.options import Options
from webdriver_manager.chrome import ChromeDriverManager


def check_file_exists(system_path):
    """Check if the given path points to an existing file."""
    return os.path.isfile(system_path)


def setup_driver(download_dir):
    """Initialize and return a Chrome WebDriver configured for PDF downloads."""
    chrome_options = Options()
    chrome_options.add_experimental_option(
        "prefs",
        {
            "download.default_directory": download_dir,
            "plugins.always_open_pdf_externally": True,
            "download.prompt_for_download": False,
        },
    )
    chrome_options.add_argument("--headless")  # Optional: disable for debugging
    return webdriver.Chrome(
        service=Service(ChromeDriverManager().install()), options=chrome_options
    )


def wait_for_new_pdf(directory, before_files, timeout=10):
    """Wait for a new PDF to appear in the directory."""
    end_time = time.time() + timeout
    while time.time() < end_time:
        current_files = set(os.listdir(directory))
        new_files = [f for f in current_files - before_files if f.endswith(".pdf")]
        if new_files:
            return os.path.join(directory, new_files[0])
    raise TimeoutError("PDF download timed out.")


# Validate a given url
def validate_url(given_url):
    return validators.url(given_url)


def process_downloads(file_path, download_dir):
    """Process the input file and download all PDFs with specified filenames."""
    os.makedirs(download_dir, exist_ok=True)
    driver = setup_driver(download_dir)

    with open(file_path, "r") as f:
        for line_num, line in enumerate(f, start=1):
            try:
                line = line.strip()
                if not line:
                    continue
                url, filename = line.split(maxsplit=1)
                output_path = os.path.join(download_dir, filename)

                if validate_url(url) is False:
                    print(f"[{line_num}] Invalid URL: {url}")
                    continue

                if check_file_exists(output_path):
                    print(f"[{line_num}] Skipped (already exists): {filename}")
                    continue

                print(f"[{line_num}] Downloading: {url}")
                before_files = set(os.listdir(download_dir))

                driver.get(url)
                downloaded_file = wait_for_new_pdf(download_dir, before_files)

                shutil.move(downloaded_file, output_path)
                print(f"[{line_num}] Saved as: {output_path}")

            except Exception as e:
                print(f"[{line_num}] Error downloading from {url}: {e}")
                continue

    driver.quit()
    print("\n✅ All downloads complete.")


if __name__ == "__main__":
    INPUT_FILE = "pdf_list.txt"
    DOWNLOAD_DIR = os.path.abspath("PDFs")
    process_downloads(INPUT_FILE, DOWNLOAD_DIR)