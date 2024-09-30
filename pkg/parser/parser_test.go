package parser

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	stmt, err := Parse("create user admin with password 'abcd'")
	fmt.Println(stmt, err)

	stmt, err = Parse("128")
	fmt.Println(stmt, err)
}
