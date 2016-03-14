package main

import (
		"fmt"
		"mock"
)

func main() {

	flag.Parse()
	args := flag.Args()

	if len(args) != 2 {
		//usage()
		fmt.Println("command not in correct format: check usage")
	}

	fileinfo := mock.PreMockChecking(args[0], args[1])

	for _, x := range fileinfo {
        mock.WriteExportedContent(x)
    }

}