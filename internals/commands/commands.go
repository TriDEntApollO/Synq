package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"github.com/TriDEntApollO/Synq/internals/utils"
	"io"
	"os"
	"strings"
)

func Help(Args []string) {
	fmt.Println("usage: synq <command> [<args>...]")
	fmt.Println()
	fmt.Println("The most commonly used synq commands are:")
	fmt.Println("   init           Initialize a new git repository or reinitialize an existing one")
	fmt.Println("   cat-file       Show various types of objects")
	fmt.Println("   hash-object    Create a blob object from a file")
	fmt.Println()
	fmt.Println("For more information on any of these commands, run 'synq help <command>'.")
}

func SynqInit(osArgs []string) {
	dirsToCreate := []string{".git/hooks", ".git/info", ".git/objects", ".git/refs", ".git/refs/heads", ".git/refs/tags"}
	headFileContents := []byte("ref: refs/heads/main\n")
	configFileContents := []byte("[core]\n\trepositoryformatversion = 0\n\tfilemode = true\n\tbare = false\n\tlogallrefupdates = true\n")
	descriptionFileContents := []byte("Unnamed repository; edit this file 'description' to name the repository.\n")

	switch len(osArgs) {
	case 2:
		if err := utils.CreateGitDir(""); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		for _, dir := range dirsToCreate {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "error creating directory: %s\n", err)
			}
		}

		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing HEAD file: %s\n", err)
		}
		if err := os.WriteFile(".git/config", configFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing config file: %s\n", err)
		}
		if err := os.WriteFile(".git/description", descriptionFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing description file: %s\n", err)
		}

		parentDir, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error getting current directory: %s\n", err)
		}
		fmt.Printf("initialized an empty git directory in: '%s.git/'\n", parentDir)

	case 3:
		// Normalise and format the dirdctory path
		parentDir, err := utils.NormalizePath(osArgs[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "fatal: error normalizing path\nerror: %s\n", err)
			os.Exit(1)
		}

		// create .git directory
		if err := utils.CreateGitDir(parentDir); err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}

		// create other correspoding direcotries
		for _, dir := range dirsToCreate {
			if err := os.MkdirAll(parentDir+"/"+dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}

		// create and write correspoding files
		if err := os.WriteFile(parentDir+"/.git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing file: %s\n", err)
		}
		if err := os.WriteFile(parentDir+"/.git/config", configFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing config file: %s\n", err)
		}
		if err := os.WriteFile(parentDir+"/.git/description", descriptionFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing description file: %s\n", err)
		}

		fmt.Printf("initialized an empty git repository in '%s.git/'\n", parentDir)
	}
}

func CatFile(Args []string) {
	// check if arguments are valid
	if len(Args) < 4 {
		fmt.Fprintf(os.Stderr, "usage: synq cat-file <options> <objec-hasht>\n")
		os.Exit(1)
	}

	objectHash := Args[3]
	if len(objectHash) != 40 {
		fmt.Fprintf(os.Stderr, "fatal: invalid object name '%s'\n", objectHash)
		os.Exit(1)
	}

	decompressedContents := utils.ReadFromGitObject(objectHash)    // read decompressed object contents
	fileContent := utils.SplitAtChar(decompressedContents, '\x00') // split after housekeeping string to get the file contents

	if Args[2] == "-p" || Args[2] == "--print" {
		fmt.Println(fileContent)
	}
}

func HashObject(Args []string) {
	if len(Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: synq hash-object [-w] <file>\n")
		os.Exit(1)
	}

	var filename string
	if len(Args) == 3 {
		filename = Args[2]
	} else {
		filename = Args[3]
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

	if err := utils.ReadFromReader(file, &strBuilder, fileInfo.Size()); err != nil { // write file contents into string builder
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
	if len(Args) > 3 {
		switch Args[2] {
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
}

func LsTree(Args []string) {
	if len(Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: synq hash-object [--name-only] <tree-hash>\n")
		os.Exit(1)
	}

	var treeHash string
	if len(Args) == 3 {
		treeHash = Args[2]
	} else {
		treeHash = Args[3]
	}

	if len(treeHash) != 40 {
		fmt.Fprintf(os.Stderr, "fatal: invalid object name '%s'\n", treeHash)
		os.Exit(1)
	}

	decompressedContents := utils.ReadFromGitObject(treeHash) // read decompressed object contents

	// verify if is a valid tree object
	if decompressedContents[0:4] != "tree" {
		fmt.Fprintf(os.Stderr, "fatal: object '%s' is not a tree\n", treeHash)
		os.Exit(1)
	}

	// print contents
	fileContent := utils.SplitAtChar(decompressedContents, '\x00') // split after housekeeping string to get the file contents

	if Args[2] == "--name-only" {
		for _, object := range utils.ParseTreeObject(fileContent, ' ', '\x00') {
			fmt.Println(object[1])
		}
	} else {
		// print each obejct one line at a time
		for _, object := range utils.ParseTreeObject(fileContent, ' ', '\x00') {
			var objectType string
			// detect type of object from the mode
			switch object[0] {
			case "100644":
				fallthrough
			case "100755":
				fallthrough
			case "120000":
				objectType = "blob"
			case "040000":
				objectType = "tree"
			case "160000":
				objectType = "commit"
			}
			fmt.Printf("%s %s %x\t%s\n", object[0], objectType, object[2], object[1])
		}
	}
}
