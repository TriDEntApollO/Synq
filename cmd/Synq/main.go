package main

import (
	"fmt"
	"github.com/TriDEntApollO/Synq/internals/commands"
	"os"
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
		commands.SynqInit(os.Args)

	case "cat-file":
		commands.CatFile(os.Args)

	case "hash-object":
		commands.HashObject(os.Args)

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command '%s'\n", command)
		os.Exit(1)
	}
}
