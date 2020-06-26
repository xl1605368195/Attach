package main

import (
	"Attach/app"
	"fmt"
)

func main() {
	vm := app.NewVirtualMachine(46126)
	err := vm.Attach()
	if err != nil {
		fmt.Printf("attach err,%v", err)
	}
	err = vm.LoadAgent("/Users/xule05/Downloads/SimpleAgent/target/simple-agent-1.0-SNAPSHOT-jar-with-dependencies.jar=xxxx")
	if err != nil {
		fmt.Printf("loadAgent err,%v", err)
	}
	vm.Detach()
}
