package main

import (
	"fmt"
	"os"
	"testing"
)

func TestGetServerNameFromPath(t *testing.T) {
	nowDir, _ := os.Getwd()
	fmt.Println(getServerNameFromPath("./", nowDir))
}