package main

import (
	"flag"
	"fmt"
	"github.com/akankshagupta1989/Morpheus/mock"
	"os"
)

func main() {

	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		//usage()
		fmt.Println("command not in correct format: check usage")
		os.Exit(1)
	}

	fileinfo := mock.PreMockChecking(args[0], args[1])

	for _, x := range fileinfo {
		mock.WriteExportedContent(x)
	}

}
