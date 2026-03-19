package reap

func (p REAPParams) debugLogf(format string, args ...interface{}) {
	if !p.DebugEnabled {
		return
	}

	log.Debugf(format, args...)
}
