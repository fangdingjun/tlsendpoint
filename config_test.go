package main

import (
	"fmt"
	"testing"
)

func TestConfig(t *testing.T) {
	c, err := loadConfig("config_example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", c)
}
