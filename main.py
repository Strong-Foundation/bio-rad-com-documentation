import os  # Import the os module for interacting with the operating system
import time  # Import time module for timestamping in logs
import fitz  # Import PyMuPDF (fitz) for PDF handling

# Define the log file path
python_log_file = (
    "python-app.log"  # Path to the log file where messages will be recorded
)


# Function to log messages to a file
def log_message(message: str):
    with open(
        python_log_file, "a", encoding="utf-8"
    ) as log:  # Open log file in append mode
        # Get the current time
        current_time = time.strftime(
            "%Y-%m-%d %H:%M:%S", time.localtime()
        )  # Format current time as YYYY-MM-DD HH:MM:SS
        # Write the message with the current time
        log.write(
            f"[{current_time}] {message}\n"
        )  # Append timestamped message to the log file


# Function to validate a single PDF file.
def validate_pdf_file(file_path):
    try:
        # Try to open the PDF using PyMuPDF
        doc = fitz.open(file_path)  # Attempt to load the PDF document

        # Check if the PDF has at least one page
        if doc.page_count == 0:  # If there are no pages in the document
            log_message(
                f"'{file_path}' is corrupt or invalid: No pages"
            )  # Log error if PDF is empty
            return False  # Indicate invalid PDF

        # If no error occurs and the document has pages, it's valid
        return True  # Indicate valid PDF
    except RuntimeError as e:  # Catching RuntimeError for invalid PDFs
        log_message(
            f"'{file_path}' is corrupt or invalid: {e}"
        )  # Log the exception message
        return False  # Indicate invalid PDF


# Remove a file from the system.
def remove_system_file(system_path):
    os.remove(system_path)  # Delete the file at the given path


# Function to walk through a directory and extract files with a specific extension
def walkGivenDirectoryAndExtractCustomFileUsingFileExtension(system_path, extension):
    matched_files = []  # Initialize list to hold matching file paths
    for root, _, files in os.walk(system_path):  # Recursively traverse directory tree
        for file in files:  # Iterate over files in current directory
            if file.endswith(extension):  # Check if file has the desired extension
                full_path = os.path.abspath(
                    os.path.join(root, file)
                )  # Get absolute path of the file
                matched_files.append(full_path)  # Add to list of matched files
    return matched_files  # Return list of all matched file paths


# Check if a file exists
def check_file_exists(system_path):
    return os.path.isfile(system_path)  # Return True if a file exists at the given path


# init function
def init():
    if check_file_exists(python_log_file):  # If the log file already exists
        # If the log file exists, remove it
        remove_system_file(
            python_log_file
        )  # Delete the existing log file to start fresh


# Get the filename and extension.
def get_filename_and_extension(path):
    return os.path.basename(
        path
    )  # Return just the file name (with extension) from a path


# Function to check if a string contains an uppercase letter.
def check_upper_case_letter(content):
    return any(
        upperCase.isupper() for upperCase in content
    )  # Return True if any character is uppercase


# Main function.
def main():
    # Initialize the log file
    init()  # Remove existing log file if present

    # Walk through the directory and extract .pdf files
    files = walkGivenDirectoryAndExtractCustomFileUsingFileExtension(
        "./PDFs", ".pdf"
    )  # Find all PDFs under ./PDFs

    # Validate each PDF file
    for pdf_file in files:  # Iterate over each found PDF

        # Check if the .PDF file is valid
        if validate_pdf_file(pdf_file) == False:  # If PDF is invalid
            # Remove the invalid .pdf file.
            remove_system_file(pdf_file)  # Delete the corrupt PDF

        # Check if the filename has an uppercase letter
        if check_upper_case_letter(
            get_filename_and_extension(pdf_file)
        ):  # If the filename contains uppercase
            # Print the location to the file.
            print(pdf_file)  # Output the PDF path to stdout
            # Print whether it matches the specs (uppercase presence)
            print(
                check_upper_case_letter(pdf_file)
            )  # Output True/False for uppercase check
            # Break so it ends out of the loop.
            break  # Exit the loop after first uppercase match


# Run the main function
main()  # Invoke main to start processing
