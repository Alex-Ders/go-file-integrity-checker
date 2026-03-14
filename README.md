# Cryptographic Integrity Checker

An asynchronous, high-performance file integrity monitor written in Go.

This project was developed as part of my portfolio for admission to cybersecurity studies. It establishes a cryptographic baseline of a target directory using SHA-256 hashing and subsequently verifies the directory to detect any unauthorized modifications, creations, or deletions.

## 🏗️ Architecture & Security Features

Instead of a simple synchronous loop, this tool is built to handle enterprise-scale directories without crashing the host system. It heavily utilizes Go's concurrency to balance speed with strict resource management:

* **Concurrency Throttling (Semaphore):** To prevent hardware interrupt storms and CPU bottlenecking, the tool uses a buffered channel acting as a semaphore, strictly limiting the system to 10 active worker goroutines at a time.
* **Thread-Safe Memory (Mutex):** A `sync.Mutex` locks the shared dictionary during baseline creation, preventing race conditions and data collisions when multiple workers attempt to write their hash results simultaneously.
* **Dynamic Context Timeouts:** To defend against "hung" processes or massive files, the hashing engine is wrapped in an asynchronous task execution block. It uses a dynamic stopwatch (`context.WithTimeout`) that allocates a baseline of 5 seconds, plus 1 additional second for every 100MB of file size. If a worker exceeds its deadline, the operation is abandoned.
* **Baseline Hardening:** Once the baseline is calculated, the tool automatically modifies the `baseline.json` file permissions to `0444` (Read-Only) to add a layer of friction against unauthorized tampering.

## 🚀 Usage

The tool operates via command-line arguments and requires three parameters to run.

**1. Establish the Baseline**

Run this command on a known-good directory. It calculates the hashes and saves them to `baseline.json`.


go run main.go baseline <directory_path>


**2. Verify Integrity**
Run this command later to scan the directory against the saved baseline.json. The tool will output alerts for:

    [!] NEW FILE DETECTED

    [!] MODIFIED FILE DETECTED

    [!] DELETED FILE DETECTED

    [+] OK (File is untouched)


go run main.go verify <directory_path>


## 🧠 Future Roadmap
As I will start my cybersecurity education, I plan to expand this tool with features like automated quarantine for new/modified files and alerting via syslog.