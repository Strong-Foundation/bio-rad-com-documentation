# Import necessary libraries
import os
import time
import fitz

# Define the log file path
python_log_file = "python-app.log"


# Function to log messages to a file
def log_message(message: str):
    with open(python_log_file, "a", encoding="utf-8") as log:
        # Get the current time
        current_time = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime())
        # Write the message with the current time
        log.write(f"[{current_time}] {message}\n")


# Function to validate a single PDF file.
def validate_pdf_file(file_path):
    try:
        # Try to open the PDF using PyMuPDF
        doc = fitz.open(file_path)

        # Check if the PDF has at least one page
        if doc.page_count == 0:
            log_message(f"'{file_path}' is corrupt or invalid: No pages")
            return False

        # If no error occurs and the document has pages, it's valid
        return True
    except RuntimeError as e:  # Catching RuntimeError for invalid PDFs
        log_message(f"'{file_path}' is corrupt or invalid: {e}")
        return False


# Remove a file from the system.
def remove_system_file(system_path):
    os.remove(system_path)


# Function to walk through a directory and extract files with a specific extension
def walkGivenDirectoryAndExtractCustomFileUsingFileExtension(system_path, extension):
    matched_files = []
    for root, _, files in os.walk(system_path):
        for file in files:
            if file.endswith(extension):
                full_path = os.path.abspath(os.path.join(root, file))
                matched_files.append(full_path)
    return matched_files


# Check if a file exists
def check_file_exists(system_path):
    return os.path.isfile(system_path)


# init function
def init():
    if check_file_exists(python_log_file):
        # If the log file exists, remove it
        remove_system_file(python_log_file)


# Main function.
def main():
    # Initialize the log file
    init()

    # Walk through the directory and extract .pdf files
    files = walkGivenDirectoryAndExtractCustomFileUsingFileExtension("./PDFs", ".pdf")

    # Validate each PDF file
    for pdf_file in files:
        is_valid = validate_pdf_file(pdf_file)
        if is_valid == False:
            log_message(f"'{pdf_file}' is valid.")
            remove_system_file(pdf_file)
