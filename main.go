package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
    "sync"
	"time"
	"context"
)

// baseline database 
const baselineFile = "baseline.json"

// hashFile takes filepath, reads file, returns SHA-256 and error
func hashFile(filePath string) (string, error) {
	//opens the physical file
	file, err := os.Open(filePath)
	if err != nil {
		return "", err 
	}
	defer file.Close() 

	// SHA-256 math engine
	hashEngine := sha256.New()

	// copies the file's contents into the math engine
	_, err = io.Copy(hashEngine, file)
	if err != nil {
		return "", err
	}

	// calculates the final SHA-256  and converts it to readable text
	hashBytes := hashEngine.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	return hashString, nil 
}

// hashFileWithContext is a wrapper that adds a timer to the old hash factory
func hashFileWithContext(ctx context.Context, filePath string) (string, error) {
	
	type result struct {
		hash string
		err  error
	}
	
	// channel that can hold exactly one 'box'
	ch := make(chan result, 1)

	// hashing is done by a background goroutine for ctx timeout monitoring (through chan)
	go func() {
		h, e := hashFile(filePath)
		ch <- result{hash: h, err: e}
	}()

	// RACE: ctx timeout against worker results
	// operation is abandoned if execution exceeds deadline
	select {
	case <-ctx.Done():
	
		return "", fmt.Errorf("hashing timed out")
		
	case res := <-ch:
	
		return res.hash, res.err
	}
}
   

func createBaseline(directory string) {
    baseline := make(map[string]string)
    
    // traffic light set up
    var wg sync.WaitGroup // stop sign
    var mu sync.Mutex     // prevents race condition, only 1 worker can write at once

    // channel which holds 10 empty 'wristbands' for the workers
    sem := make(chan struct{}, 10)

    fmt.Println("[*] Calculating baseline fingerprints concurrently...")

    filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
        if !info.IsDir() {
            
            wg.Add(1)

            // block until slot is available in semaphore for throttling
            select {
			case sem <- struct{}{}:
			case <-time.After(10 * time.Second):
    		fmt.Printf("[-] TIMEOUT: Manager stuck for 10s at %s. Possible deadlock!\n", path)
    		return fmt.Errorf("timeout reached") 
			}		
            // goroutine is launched 
            go func(filePath string) {

                // defers the subtraction so it ticks down when the thread finishes
                defer wg.Done() 
				defer func() { <-sem }()

				
				fileSizeMB := info.Size() / (1024 * 1024)

				// 5 seconds baseline + 1 second for every 100MB
				// time.Duration() to translate it to a time based value
				timeoutDuration := time.Duration(5+(fileSizeMB/100)) * time.Second

				// dynamic stopwatch is applied
				ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
				defer cancel()

				// tells hashFile to respect the clock 
    			hash, err := hashFileWithContext(ctx, filePath)

				if err != nil {
        			fmt.Printf("[-] Worker failed/timed out on %s: %v\n", filePath, err)
        		return // worker dies, returns wristband
    			}

					
                    // lock 'door', write to dictionary, unlock 'door'
                    mu.Lock()
                    baseline[filePath] = hash
                    mu.Unlock()
                
            }(path) 
        }
        return nil
    })
    // tells the main program to wait here until the counter hits 0
    wg.Wait()

    // completed baseline dictionary is saved
    file, _ := os.Create(baselineFile)
    encoder := json.NewEncoder(file)
    encoder.Encode(baseline)
    file.Close() 

    err := os.Chmod(baselineFile, 0444)
    if err != nil {
        fmt.Println("[-] Warning: Could not set file to read-only.")
    } else {
        fmt.Println("[+] Baseline hardened: File is now Read-Only.")
    }
}

// verifyIntegrity checks the directory against the saved baseline
func verifyIntegrity(directory string) {
	// reads saved JSON database from the hard drive
	file, err := os.Open(baselineFile)
	if err != nil {
		fmt.Println("[-] Error: No baseline found. Please create one first.")
		return
	}
	defer file.Close()

	// decodes the JSON back into a Go dictionary
	var baseline map[string]string
	decoder := json.NewDecoder(file)
	decoder.Decode(&baseline)

	fmt.Println("[*] Verifying file integrity...")

	// scan directory again
	filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			currentHash, _ := hashFile(path)
			
			// checks if the file existed in the baseline
			originalHash, exists := baseline[path]

			if !exists {
				
				fmt.Printf("[!] NEW FILE DETECTED: %s\n", path)
			} else if currentHash != originalHash {
				
				fmt.Printf("[!] MODIFIED FILE DETECTED: %s\n", path)
			} else {
				
				fmt.Printf("[+] OK: %s\n", path)
			}
			
			// removes file from baseline dictionary copy to make sure its been checked
			delete(baseline, path) 
		}
		return nil
	})
     // checks if theres deleted files
	for deletedFile := range baseline {
		fmt.Printf("[!] DELETED FILE DETECTED: %s\n", deletedFile)
	}
}

// the 'ignition key', reads user input and runs the tool
func main() {
	//os.Args holds the user input, length check is performed
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <baseline|verify> <directory_path>")
		return
	}

	mode := os.Args[1]      // 'baseline' or 'verify'
	directory := os.Args[2] // folder user wants to scan

	// if/else statement which controls whether the baseline or verify functions become active
	if mode == "baseline" {
		createBaseline(directory)
	} else if mode == "verify" {
		verifyIntegrity(directory)
	} else {
		fmt.Println("[-] Unknown mode. Use 'baseline' or 'verify'.")
	}
}