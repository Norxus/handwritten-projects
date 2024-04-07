package main

import (
	"github.com/fatedier/golib/crypto"
)

func main() {
	crypto.DefaultSalt = "frp"
	Execute()
}
