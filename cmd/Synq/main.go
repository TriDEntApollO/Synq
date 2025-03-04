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
		commands.Help(os.Args)

	case "init":
		commands.SynqInit(os.Args)

	case "cat-file":
		commands.CatFile(os.Args)

	case "hash-object":
		commands.HashObject(os.Args)

	case "ls-tree":
		commands.LsTree(os.Args)

	default:
		fmt.Fprintf(os.Stderr, "synq: '%s' is not a valid command. See 'synq help' for more info.\n", command)
		os.Exit(1)
	}
}
