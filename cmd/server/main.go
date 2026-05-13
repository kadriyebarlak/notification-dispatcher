package main

import (
	"fmt"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

func main() {
	fmt.Println("notification dispatcher starting...")

	var s domain.EventStatus = "something_random"
	fmt.Println(s)
}
