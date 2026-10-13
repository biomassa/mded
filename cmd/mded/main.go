// Command mded is a markdown viewer and editor for the terminal.
package main

import (
	"fmt"

	"github.com/biomassa/mded/internal/version"
)

func main() { fmt.Println("mded", version.String()) }
