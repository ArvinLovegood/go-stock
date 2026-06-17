package main

import (
	"github.com/bensema/gotdx"
)

func main() {
	gotdx.New(gotdx.WithAutoSelectFastest(true))
}
