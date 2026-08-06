//go:build !windows

package main

import "github.com/gin-gonic/gin"

var listUNCServerShares = listUNCServerSharesUnsupported

func listUNCServerSharesUnsupported(serverRoot string) ([]gin.H, error) {
	return nil, errUNCServerShareDiscoveryUnsupported
}
