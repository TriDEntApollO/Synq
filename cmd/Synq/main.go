package main

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	// Check if valid amount of arguments have been pased
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: synq <command> [<args>...]\n")
		os.Exit(1)
	}

	// Check for what command has been passed
	switch command := os.Args[1]; command {
	case "help":
		fmt.Println("synq is a simple git implementation")

	case "init":
		switch len(os.Args) {
		case 2:
			for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					fmt.Fprintf(os.Stderr, "error creating directory: %s\n", err)
				}
			}

			headFileContents := []byte("ref: refs/heads/main\n")
			if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing file: %s\n", err)
			}

			currDir, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error getting current directory: %s\n", err)
			}
			fmt.Printf("initialized an empty git directory in: '%s.git'\n", currDir)

		case 3:
			for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
				if err := os.MkdirAll(os.Args[2]+"/"+dir, 0755); err != nil {
					fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
				}
			}

			headFileContents := []byte("ref: refs/heads/main\n")
			if err := os.WriteFile(os.Args[2]+"/.git/HEAD", headFileContents, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "error writing file: %s\n", err)
			}

			fmt.Printf("initialized an empty git repository in '%s'\n", os.Args[2])
		}

	case "cat-file":
		// check if arguments are valid
		if len(os.Args) < 4 {
			fmt.Fprintf(os.Stderr, "usage: synq cat-file <options> <object>\n")
			os.Exit(1)
		}

		sha1 := os.Args[3]
		if len(sha1) != 40 {
			fmt.Fprintf(os.Stderr, "fatal: invalid object name '%s'\n", sha1)
			os.Exit(1)
		}

		// construct the object directory path and open the file
		path := fmt.Sprintf(".git/objects/%v/%v", sha1[0:2], sha1[2:])
		objectFile, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "fatal: object '%s' does nto exist\n", sha1)
			} else {
				fmt.Fprintf(os.Stderr, "fatal: error opening file: %s\n", err)
			}
			os.Exit(1)
		}

		// decompress the file contents
		zlibObjectReader, err := zlib.NewReader(objectFile)
		if err != nil {
			fmt.Println("fatal: error creating zlib reader:", err)
			objectFile.Close()
			os.Exit(1)
		}

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
		if err := ReadFromObject(zlibObjectReader, &strBuilder, fileInfo.Size()); err != nil { // write file contents into string builder
			fmt.Fprintln(os.Stderr, err)
			objectFile.Close()
			zlibObjectReader.Close()
			os.Exit(1)

		}

		decompressedContents := strBuilder.String()      // build the string from bytes
		fileContent := SplitAtNull(decompressedContents) // split after housekeeping string to get the file contents

		if os.Args[2] == "-p" || os.Args[2] == "--print" {
			fmt.Println(fileContent)
		}

		objectFile.Close()
		zlibObjectReader.Close()

	case "hash-object":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "usage: synq hash-object [-w] <file>\n")
			os.Exit(1)
		}

		var filename string
		if len(os.Args) == 3 {
			filename = os.Args[2]
		} else {
			filename = os.Args[3]
		}

		file, err := os.Open(filename)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "fatal: file '%s' does nto exist\n", filename)
			} else {
				fmt.Fprintf(os.Stderr, "fatal: error opening file: %s\n", err)
			}
			os.Exit(1)
		}

		//build the blob object contents
		var strBuilder strings.Builder
		strBuilder.WriteString("blob ") // write blob header with a trailing space into string builder

		fileInfo, err := file.Stat()
		if err != nil {
			fmt.Fprintf(os.Stderr, "fatal: cannot determine file size: %s\n", err)
			file.Close()
			os.Exit(1)
		}
		fmt.Fprintf(&strBuilder, "%d\x00", fileInfo.Size()) // write size of file with trailing \x00 into string builder

		if err := ReadFromObject(file, &strBuilder, fileInfo.Size()); err != nil { // write file contents into string builder
			fmt.Fprintln(os.Stderr, err)
			file.Close()
			os.Exit(1)

		}

		// generate the hash
		hasher := sha1.New()
		if _, err := io.WriteString(hasher, strBuilder.String()); err != nil {
			fmt.Fprintf(os.Stderr, "fatal: error hashing file: %s\n", err)
			file.Close()
			os.Exit(1)
		}
		fileHash := fmt.Sprintf("%x", hasher.Sum(nil))

		// check for flags
		if len(os.Args) > 3 {
			switch os.Args[2] {
			case "-w": // if falg is -w or --write
				fallthrough
			case "--write":
				//compress the blob contents into zlib data
				var compressedData bytes.Buffer
				writer := zlib.NewWriter(&compressedData)

				if _, err := writer.Write([]byte(strBuilder.String())); err != nil { // Convert Builder to string and write
					fmt.Fprintf(os.Stderr, "fatal: error compressing file: %s\n", err)
					file.Close()
					writer.Close()
					os.Exit(1)
				}
				writer.Close() // Ensure all data is flushed

				// create the necesarry directory if it doesn't exist
				if err := os.MkdirAll(fmt.Sprintf(".git/objects/%v", fileHash[0:2]), 0755); err != nil {
					fmt.Fprintf(os.Stderr, "fatal: error creating directory: %s\n", err)
					file.Close()
					os.Exit(1)
				}
				// create the object file and write the contents to it
				objectFile := fmt.Sprintf(".git/objects/%v/%v", fileHash[:2], fileHash[2:])
				if err := os.WriteFile(objectFile, compressedData.Bytes(), 0644); err != nil {
					fmt.Fprintf(os.Stderr, "fatal: error writing file: %s\n", err)
					file.Close()
					os.Exit(1)
				}

			default:
				break
			}
		}

		fmt.Println(fileHash)
		file.Close()

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command '%s'\n", command)
		os.Exit(1)
	}
}

func ReadFromObject(source io.Reader, strBuilder *strings.Builder, fileSize int64) error {
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

func SplitAtNull(str string) string {
	for i, c := range str {
		if c == 0 {
			return str[i+1:]
		}
	}
	return str
}
