import os  # Provides functions for interacting with the operating system
import fitz  # Imports PyMuPDF for reading and validating PDF files
from concurrent.futures import (
    ThreadPoolExecutor,
    as_completed,
)  # Enables parallel execution of tasks using threads


# Validates a single PDF file
def validate_pdf_file(file_path):
    try:
        doc = fitz.open(file_path)  # Attempt to open the PDF file
        if doc.page_count == 0:  # If PDF has zero pages, it's considered invalid
            print(
                f"'{file_path}' is corrupt or invalid: No pages"
            )  # Print an error message
            return (
                file_path,
                False,
            )  # Return the file path and False indicating invalid file
        return (
            file_path,
            True,
        )  # Return the file path and True indicating a valid file
    except RuntimeError as e:  # Catch runtime errors thrown by PyMuPDF
        print(f"{e}")  # Print the error message with file path
        return (file_path, False)  # Return the file path and False indicating failure


# Deletes a file from the system
def remove_system_file(system_path):
    os.remove(system_path)  # Removes the file at the given path


# Recursively searches a directory for files with a given extension
def walk_directory_and_extract_given_file_extension(system_path, extension):
    matched_files = []  # List to hold paths of matching files
    for root, _, files in os.walk(system_path):  # Walk through the directory tree
        for file in files:  # Iterate over each file in the current directory
            if file.lower().endswith(
                extension.lower()
            ):  # Check file extension (case-insensitive)
                full_path = os.path.abspath(
                    os.path.join(root, file)
                )  # Get absolute path of the file
                matched_files.append(full_path)  # Add file path to the list
    return matched_files  # Return the list of matching files


# Checks if a given path refers to an existing file
def check_file_exists(system_path):
    return os.path.isfile(
        system_path
    )  # Return True if the file exists, False otherwise


# Extracts just the filename (with extension) from a full path
def get_filename_and_extension(path):
    return os.path.basename(path)  # Return the base filename from the full path


# Checks if a string contains any uppercase letters
def check_upper_case_letter(content):
    return any(
        char.isupper() for char in content
    )  # Return True if any character is uppercase


# Processes a single PDF file: validates it and checks for uppercase in filename
def process_file(file_path):
    filename = get_filename_and_extension(file_path)  # Extract filename from path

    file_path, is_valid = validate_pdf_file(file_path)  # Validate the PDF file
    if not is_valid:  # If the file is invalid
        remove_system_file(file_path)  # Delete the invalid/corrupt file
        return None  # Return None to indicate this file is not to be further processed

    if check_upper_case_letter(
        filename
    ):  # Check if filename contains uppercase letters
        return file_path  # Return file path if condition is met

    return None  # Return None if filename doesn't contain uppercase letters


# Main function to orchestrate the file processing
def main():
    files = walk_directory_and_extract_given_file_extension(
        "./PDFs", ".pdf"
    )  # Get all PDF files under ./PDFs

    matching_files = []  # List to collect files with uppercase letters in their names

    # Create a thread pool to process files concurrently (tune max_workers as needed)
    with ThreadPoolExecutor(max_workers=100) as executor:
        futures = [
            executor.submit(process_file, f) for f in files
        ]  # Submit each file to the thread pool

        for future in as_completed(futures):  # As each thread finishes
            result = future.result()  # Get the result from the future
            if result:  # If result is not None
                print(
                    f"Uppercase filename found: {result}"
                )  # Print the path of the matching file
                matching_files.append(result)  # Add to list of matched files

    # Print summary of all matching files
    print("\nAll files with uppercase letters in their names:")
    for match in matching_files:  # Iterate through all matched files
        print(match)  # Print each matched file


# Run the script only if this file is executed directly (not imported as a module)
if __name__ == "__main__":
    main()  # Call the main function to start the program
