package main

import (
	"fmt"
)

func outputConsole(q <-chan []*string) {
	for m := range q {
		for _, l := range m {
			fmt.Println(*l)
		}
	}
}
