package output

import (
	"fmt"
)

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
