package utils

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func NormalizePath(input string) (string, error) {
	input = strings.Trim(input, `"`) // Windows Fix: Trim any accidental trailing quote (") if present

	// trim any trailing slashes (`/` or `\`) except for root paths
	if len(input) > 1 {
		input = strings.TrimRight(input, "/\\")
	}

	// expand `~` to the user's home directory
	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err // Return an error if home directory cannot be retrieved
		}
		input = strings.Replace(input, "~", home, 1)
	}

	// convert to an absolute path
	absPath, err := filepath.Abs(input)
	if err != nil {
		return "", err // Return error if unable to get absolute path
	}

	cleanPath := filepath.Clean(absPath)     // normalize the path (removes redundant separators, `.` and `..`)
	finalPath := filepath.ToSlash(cleanPath) // convert to Unix-style slashes (`/`), even on Windows

	return finalPath, nil
}

func CreateGitDir(parent string) error {
	var dirName string

	// set .git path accrodingly if parent path is provided or not
	if parent == "" {
		dirName = ".git"
	} else {
		dirName = parent + "/.git"
	}

	// Check if .git already exists
	if _, err := os.Stat(dirName); err == nil {
		return fmt.Errorf("synq: already a git repository (%s already exists)", dirName)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("fatal: error checking .git existence\nerror: %v", err)
	}

	// Create the .git directory with 0755 (ignored on Windows)
	err := os.MkdirAll(dirName, 0755)
	if err != nil {
		return fmt.Errorf("fatal: error creating .git directory\nerror: %v", err)
	}

	// Handle Windows-specific logic
	if runtime.GOOS == "windows" {
		// Set .git as hidden
		path, err := syscall.UTF16PtrFromString(dirName)
		if err != nil {
			return fmt.Errorf("fatal: error converting path\nerror: %v", err)
		}
		err = syscall.SetFileAttributes(path, syscall.FILE_ATTRIBUTE_HIDDEN)
		if err != nil {
			return fmt.Errorf("fatal: error setting hidden attribute to '.git' directory\nerror: %v", err)
		}
	}

	return nil
}

func ReinitializeGitDir() error {
	return nil
}

func ReadFromReader(source io.Reader, strBuilder *strings.Builder, fileSize int64) error {
	const fileSizeThreshold = 10 * 1024 * 1024 // 10MB

	// read decompressed data
	if fileSize < fileSizeThreshold { // if file size less than 10MB read all at once
		data, err := io.ReadAll(source)
		if err != nil {
			return fmt.Errorf("error reading decompressed data: %w", err)
		}
		strBuilder.Write(data)
	} else { // if file size larger than 10MB read in chunks
		buf := make([]byte, 4096) // 4KB chunk size
		for {
			n, err := source.Read(buf)
			if n > 0 {
				strBuilder.Write(buf[:n])
			}
			if err != nil {
				if err == io.EOF {
					break // end of file
				}
				return fmt.Errorf("error reading decompressed data: %s\n", err)
			}
		}
	}

	return nil
}

func ReadFromGitObject(treeHash string) string {
	// construct the object directory path and open the file
	path := fmt.Sprintf(".git/objects/%v/%v", treeHash[0:2], treeHash[2:])
	objectFile, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "fatal: object '%s' does nto exist\n", treeHash)
		} else {
			fmt.Fprintf(os.Stderr, "fatal: error opening file: %s\n", err)
		}
		os.Exit(1)
	}
	defer objectFile.Close()

	// decompress the file contents
	zlibObjectReader, err := zlib.NewReader(objectFile)
	if err != nil {
		fmt.Println("fatal: error creating zlib reader:", err)
		objectFile.Close()
		os.Exit(1)
	}
	defer objectFile.Close()

	// determine file size
	fileInfo, err := objectFile.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: cannot determine file size: %s\n", err)
		objectFile.Close()
		zlibObjectReader.Close()
		os.Exit(1)
	}

	// read decompressed data
	var strBuilder strings.Builder
	if err := ReadFromReader(zlibObjectReader, &strBuilder, fileInfo.Size()); err != nil { // write file contents into string builder
		fmt.Fprintln(os.Stderr, err)
		objectFile.Close()
		zlibObjectReader.Close()
		os.Exit(1)
	}

	return strBuilder.String()
}

func SplitAtChar(str string, char rune) string {
	for i, c := range str {
		if c == char { // checks if current character in the string is the one passed as arg
			return str[i+1:] // resturn from after ith index
		}
	}
	return str // else return the whole string
}

func ParseTreeObject(str string, start rune, end rune) [][3]string {
	var arr [][3]string // Slice to store parsed objects
	var x, y int = 0, 0 // `x` marks start of name, `y` marks start of hash

	for i, char := range str {
		if char == start {
			x = i + 1 // Move past the `start` character
			// Ensure mode is a 6-digit zero-padded string
			mode := fmt.Sprintf("%06s", str[y:i])
			arr = append(arr, [3]string{mode, "", ""}) // Store mode (from `y` to `i`), empty placeholders for name & hash
		}
		if char == end {
			n := len(arr) - 1        // Get the last inserted object
			y = i + 21               // Move `y` to start of next object after the 20-byte hash
			arr[n][1] = str[x:i]     // Store object name (between `x` and `i`)
			arr[n][2] = str[i+1 : y] // Store object hash (20 bytes after `i`)
		}
	}
	return arr // Return parsed objects
}
