package install

func Rollback(target Target) error { return restoreLatest(target.Profile) }
