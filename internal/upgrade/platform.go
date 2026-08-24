package upgrade

import "runtime"

// Platform identifies the operating system and architecture an asset must match.
type Platform struct {
	OS   string
	Arch string
}

// CurrentPlatform returns the platform of the running process.
func CurrentPlatform() Platform {
	return Platform{OS: runtime.GOOS, Arch: runtime.GOARCH}
}

func (p Platform) String() string { return p.OS + "/" + p.Arch }
