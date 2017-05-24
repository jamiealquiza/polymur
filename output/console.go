// Package output console.go
// writes Polymur output to console.
package output

import (
	"fmt"
)

// Console reads from the destination queue and
// prints the datapoints to console.
func Console(q <-chan []*string) {
batch:
	for m := range q {
		for _, l := range m {
			if l == nil {
				break batch
			}
			fmt.Println(*l)
		}
	}
}
