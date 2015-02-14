package main

import (
	"flag"
	"fmt"
	"github.com/goura/spotmc"
)

var flagInitscript = flag.Bool("rhinitscript", false, "generate dummy initscript")

func main() {
	flag.Parse()
	if *flagInitscript {
		// output the initscript and exit
		data, err := Asset("data/initscript.sh")
		if err != nil {
			panic(err)
		}
		fmt.Print(string(data))
	} else {
		spotmc.Main()
	}
}
