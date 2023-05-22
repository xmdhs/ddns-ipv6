package ipv6

import (
	"context"
	"fmt"
	"testing"
)

func TestGetIpv6(t *testing.T) {
	s, err := GetIpv6(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(s)
}
