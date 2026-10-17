//go:build !js

package client

func newCloud() Cloud { return noCloud{} }
