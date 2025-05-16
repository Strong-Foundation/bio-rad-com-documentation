# 📄 Bio-Rad Documentation Archive

This repository contains an automated archive of documentation PDFs sourced from [bio-rad.com](https://www.bio-rad.com). It includes manuals, certificates, datasheets, safety information, and other official resources related to Bio-Rad products.

---

## 🗂 Contents

- `PDFs/` — All downloaded and validated PDF documentation.
- `main.go` — Script that crawls and downloads PDF files from the Bio-Rad website.
- `main.py` — Script that validates downloaded PDFs (e.g., checks integrity, removes corrupted or non-PDF files).
- `requirements.txt` — Python dependencies for running `main.py`.
- `.gitignore` — Git exclusions.
- `LICENSE` — Project license (MIT).

---

## ⚙️ How It Works

### 1. PDF Downloading (`main.go`)

The Go script is responsible for:

- Crawling Bio-Rad’s documentation endpoints.
- Downloading all reachable PDF files into the `PDFs/` directory.

Run it with:

```bash
go run main.go
```

---

### 2. PDF Validation & Cleanup (`main.py`)

The Python script:

- Scans the `PDFs/` directory.
- Validates the file format and integrity.
- Removes files that are corrupted, incomplete, or incorrectly labeled.

Run it with:

```bash
pip install -r requirements.txt
python main.py
```

---

## 🧪 Types of Documents Collected

- Product User Manuals
- Certificates of Analysis (CoAs)
- Safety Data Sheets (SDS)
- Application Guides
- ISO Compliance Certificates
- Technical Specifications

These documents are essential for laboratories and researchers working with Bio-Rad equipment and reagents.

---

## 🔐 Legal Notice

This project is intended for archival and educational purposes. All documents remain the intellectual property of Bio-Rad Laboratories. For official use or the most up-to-date documentation, always refer to [bio-rad.com](https://www.bio-rad.com).

---

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
